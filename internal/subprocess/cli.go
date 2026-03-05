package subprocess

import (
	"bufio"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/cli"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/errors"
)

const (
	// maxScanTokenSize is the maximum buffer size for reading CLI output lines.
	maxScanTokenSize = 1024 * 1024 // 1MB
	// maxStderrBufferSize is the maximum size for the stderr buffer.
	// Stderr reading continues indefinitely (callback receives all lines),
	// but the buffer stops growing after this limit to prevent unbounded memory usage.
	maxStderrBufferSize = 10 * 1024 * 1024 // 10MB
)

// CLITransport implements Transport by spawning a Claude CLI subprocess.
type CLITransport struct {
	log            *slog.Logger
	options        *config.Options
	prompt         string
	cliPath        string
	args           []string
	env            []string
	cwd            string
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdout         io.ReadCloser
	stderr         io.ReadCloser
	stderrCallback func(string) // Callback for streaming stderr output
	mu             sync.Mutex   // Protects stdin writes
	isStreaming    bool         // Whether this transport is in streaming mode
	closing        bool         // Whether Close() has been called (intentional shutdown)
	stdinClosed    bool         // Whether stdin was closed (e.g., due to context cancellation)
}

// Compile-time verification that CLITransport implements the Transport interface.
var _ config.Transport = (*CLITransport)(nil)

// NewCLITransport creates a new CLI transport with the given prompt and options.
//
// The logger is used for operation tracking and debugging. It will receive
// debug, info, warn, and error messages during transport operations.
//
// CLI discovery is deferred to Start(), which searches for the Claude CLI binary
// in the following order:
//  1. The explicit path in options.CliPath (if provided)
//  2. The system PATH
//  3. Common installation directories (/usr/local/bin, /usr/bin, ~/.local/bin)
//
// Start() returns CLINotFoundError if the CLI binary cannot be located.
func NewCLITransport(
	log *slog.Logger,
	prompt string,
	options *config.Options,
) *CLITransport {
	return NewCLITransportWithMode(log, prompt, options, false)
}

// NewCLITransportWithMode creates a new CLI transport with explicit streaming mode control.
//
// When isStreaming is true, the transport uses --input-format stream-json and keeps
// stdin open for sending multiple messages. The prompt is not passed on the command line
// in streaming mode - instead, messages are sent via stdin using SendMessage.
//
// When isStreaming is false (default), the transport uses --print mode with the prompt
// passed on the command line for one-shot queries.
//
// CLI discovery is deferred to Start() where it can use the caller's context.
func NewCLITransportWithMode(
	log *slog.Logger,
	prompt string,
	options *config.Options,
	isStreaming bool,
) *CLITransport {
	return &CLITransport{
		log:            log.With("component", "cli_transport"),
		options:        options,
		prompt:         prompt,
		stderrCallback: options.Stderr,
		isStreaming:    isStreaming,
	}
}

// Start starts the CLI subprocess.
//
// This method discovers the Claude CLI binary, builds command arguments,
// and spawns the process with the configured environment variables.
// It sets up stdin, stdout, and stderr pipes for communication.
//
// Returns CLINotFoundError if the CLI binary cannot be located,
// or ConnectionError if the process fails to start.
func (t *CLITransport) Start(ctx context.Context) error {
	t.log.Info("Starting Claude CLI subprocess")

	// Discover CLI binary
	discoverer := cli.NewDiscoverer(&cli.Config{
		CliPath: t.options.CliPath,
		Logger:  t.log,
	})

	cliPath, err := discoverer.Discover(ctx)
	if err != nil {
		return fmt.Errorf("discover CLI: %w", err)
	}

	t.cliPath = cliPath

	// Build command arguments
	t.args = cli.BuildArgs(t.prompt, t.options, t.isStreaming)
	t.log.Debug("Built command arguments", "args", t.args)

	// Build environment
	t.env = cli.BuildEnvironment(t.options)

	// Set working directory
	t.cwd = t.options.Cwd
	if t.cwd == "" {
		t.cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	t.log.Debug("Set working directory", "cwd", t.cwd)

	//nolint:gosec // G204: Subprocess launching with dynamic args is expected for CLI invocation
	cmd := exec.CommandContext(ctx, t.cliPath, t.args...)
	cmd.Dir = t.cwd
	cmd.Env = t.env

	// Set up stdin pipe for sending messages
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.log.Error("Failed to create stdin pipe", "error", err)

		return &errors.CLIConnectionError{Err: fmt.Errorf("stdin pipe: %w", err)}
	}

	t.stdin = stdin

	// Set up stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.log.Error("Failed to create stdout pipe", "error", err)

		return &errors.CLIConnectionError{Err: fmt.Errorf("stdout pipe: %w", err)}
	}

	t.stdout = stdout

	// Set up stderr pipe for error messages
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.log.Error("Failed to create stderr pipe", "error", err)

		return &errors.CLIConnectionError{Err: fmt.Errorf("stderr pipe: %w", err)}
	}

	t.stderr = stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		t.log.Error("Failed to start CLI process", "error", err)

		return &errors.CLIConnectionError{Err: fmt.Errorf("start process: %w", err)}
	}

	t.cmd = cmd
	t.log.Info("Claude CLI subprocess started successfully", "pid", cmd.Process.Pid)

	return nil
}

// ReadMessages reads JSON messages from the CLI stdout.
//
// This method starts a goroutine that reads line-delimited JSON from the
// CLI process stdout. Each line is parsed as a JSON object and sent to
// the messages channel.
//
// The goroutine exits when:
//   - The CLI process terminates
//   - The context is cancelled
//   - An unrecoverable error occurs
//
// Parse errors for individual messages are sent to the error channel but
// do not stop message processing. The goroutine closes both channels when
// it exits.
func (t *CLITransport) ReadMessages(
	ctx context.Context,
) (<-chan map[string]any, <-chan error) {
	messages := make(chan map[string]any)
	errs := make(chan error, 1)

	// Start stderr streaming goroutine if callback is set
	var stderrWg sync.WaitGroup

	var stderrBuffer strings.Builder

	var stderrMu sync.Mutex

	// Always buffer stderr for error reporting (must complete reads before Wait())
	// See: https://pkg.go.dev/os/exec#Cmd.StderrPipe

	stderrWg.Go(func() {
		// Simple scanner loop - relies on process kill to close pipes and unblock Scan().
		// No nested goroutine needed: when Close() kills the process, the OS closes all
		// pipes, which reliably returns from blocked Read() calls.
		scanner := bufio.NewScanner(t.stderr)
		for scanner.Scan() {
			// Check context between lines for cooperative cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()

			// Buffer stderr for error reporting (capped at maxStderrBufferSize)
			stderrMu.Lock()

			if stderrBuffer.Len() < maxStderrBufferSize {
				if stderrBuffer.Len() > 0 {
					stderrBuffer.WriteString("\n")
				}

				stderrBuffer.WriteString(line)
			}

			stderrMu.Unlock()

			// Invoke callback if set
			if t.stderrCallback != nil {
				t.stderrCallback(line)
			}
		}

		// Log scanner errors (don't fail - process may have exited)
		if err := scanner.Err(); err != nil {
			t.log.Debug("Stderr scanner error", "error", err)
		}
	})

	go func() {
		defer close(messages)
		defer close(errs)
		defer t.log.Debug("ReadMessages goroutine stopped")

		scanner := bufio.NewScanner(t.stdout)
		// Set large buffer for big messages
		buf := make([]byte, maxScanTokenSize)
		scanner.Buffer(buf, maxScanTokenSize)

		messageCount := 0

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				t.log.Debug("Context cancelled during scan", "error", ctx.Err())

				errs <- ctx.Err()

				return
			default:
			}

			line := scanner.Bytes()

			var msg map[string]any

			if err := json.Unmarshal(line, &msg); err != nil {
				t.log.Debug("Failed to unmarshal JSON message", "error", err, "message", string(line))

				errs <- &errors.CLIJSONDecodeError{
					RawData: string(line),
					Err:     err,
				}

				continue
			}

			messageCount++
			t.log.Debug("Received message from CLI", "message_count", messageCount)

			select {
			case messages <- msg:
			case <-ctx.Done():
				t.log.Debug("Context cancelled during message send", "error", ctx.Err())

				errs <- ctx.Err()

				return
			}
		}

		if err := scanner.Err(); err != nil {
			t.log.Error("Scanner error while reading CLI output", "error", err)

			errs <- fmt.Errorf("scanner error: %w", err)
		}

		// Wait for stderr goroutine before process wait
		stderrWg.Wait()

		// Wait for process to exit and capture any errors
		t.log.Debug("Waiting for CLI process to exit")

		if err := t.cmd.Wait(); err != nil {
			// Check if this is an intentional shutdown
			t.mu.Lock()
			isClosing := t.closing
			t.mu.Unlock()

			if isClosing {
				t.log.Debug("CLI process terminated during shutdown")

				return
			}

			// Use buffered stderr for error reporting (cleaned of Bun source context)
			stderrMu.Lock()

			stderrOutput := cleanStderr(stderrBuffer.String())

			stderrMu.Unlock()

			exitCode := 0

			if exitErr, ok := stderrors.AsType[*exec.ExitError](err); ok {
				exitCode = exitErr.ExitCode()
			}

			t.log.Error("CLI process exited with error", "exit_code", exitCode, "stderr", stderrOutput)

			errs <- &errors.ProcessError{
				ExitCode: exitCode,
				Stderr:   stderrOutput,
				Err:      err,
			}
		} else {
			t.log.Info("CLI process exited successfully")
		}
	}()

	return messages, errs
}

// SendMessage sends a JSON message to the CLI stdin.
//
// The data should be a complete JSON message followed by a newline.
// This method is safe for concurrent use and respects context cancellation
// even during blocking writes.
//
// If context is cancelled during a blocked write, stdin is closed to unblock
// the goroutine (safe since Go 1.9+). Subsequent calls will return ErrStdinClosed.
func (t *CLITransport) SendMessage(ctx context.Context, data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stdin == nil {
		return errors.ErrTransportNotConnected
	}

	if t.stdinClosed {
		return errors.ErrStdinClosed
	}

	// Check context before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	t.log.Debug("Sending message to CLI", "data_len", len(data))

	// Ensure data ends with newline
	// Use explicit copy to avoid mutating caller's backing array if slice has spare capacity
	if len(data) == 0 || data[len(data)-1] != '\n' {
		newData := make([]byte, len(data)+1)
		copy(newData, data)
		newData[len(data)] = '\n'
		data = newData
	}

	// Write in goroutine to respect context cancellation
	done := make(chan error, 1)

	go func() {
		_, err := t.stdin.Write(data)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.log.Error("Failed to write message to CLI", "error", err)

			return fmt.Errorf("write to stdin: %w", err)
		}

		t.log.Debug("Message sent successfully")

		return nil

	case <-ctx.Done():
		t.log.Debug("Context cancelled during write, closing stdin")
		// Close stdin to unblock the blocked Write (safe since Go 1.9+)
		if t.stdin != nil {
			_ = t.stdin.Close()
			t.stdinClosed = true
		}
		// Wait for goroutine to exit with timeout to prevent leak
		select {
		case <-done:
			// Write goroutine exited cleanly
		case <-time.After(1 * time.Second):
			t.log.Warn("Write goroutine did not exit after stdin close, potential leak")
		}

		return ctx.Err()
	}
}

// IsReady checks if the transport is ready for communication.
//
// Returns true if the CLI process is running and stdin is open.
func (t *CLITransport) IsReady() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.cmd != nil && t.cmd.Process != nil && t.stdin != nil
}

// EndInput ends the input stream (closes stdin for process transports).
//
// This signals to the CLI that no more input will be sent. The CLI process
// will continue processing any pending input and then exit normally.
func (t *CLITransport) EndInput() error {
	return t.CloseStdin()
}

// CloseStdin closes the stdin pipe to signal end of input in streaming mode.
//
// This is used in streaming mode to indicate that no more messages will be sent.
// The CLI process will continue processing any pending input and then exit normally.
func (t *CLITransport) CloseStdin() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stdin != nil && !t.stdinClosed {
		t.log.Debug("Closing stdin pipe")

		err := t.stdin.Close()
		t.stdinClosed = true
		t.stdin = nil

		return err
	}

	return nil
}

// Close terminates the CLI process.
//
// This forcefully kills the CLI process using SIGKILL. It's safe to call
// Close multiple times or on an already-terminated process.
func (t *CLITransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closing = true
	t.stdinClosed = true

	if t.cmd != nil && t.cmd.Process != nil {
		t.log.Debug("Killing CLI process", "pid", t.cmd.Process.Pid)

		if err := t.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("kill CLI process (pid %d): %w", t.cmd.Process.Pid, err)
		}
	}

	return nil
}

// cleanStderr parses and cleans stderr output from the CLI.
// Bun includes minified source context in error output which is not useful.
// This extracts just the error message and stack trace.
func cleanStderr(stderr string) string {
	if stderr == "" {
		return ""
	}

	var cleaned strings.Builder

	lines := strings.SplitSeq(stderr, "\n")

	for line := range lines {
		// Skip Bun source context lines (format: "1234 | <minified code>")
		trimmed := strings.TrimSpace(line)
		if isSourceContextLine(trimmed) {
			continue
		}

		// Keep error messages, stack traces, and other useful output
		if cleaned.Len() > 0 {
			cleaned.WriteString("\n")
		}

		cleaned.WriteString(line)
	}

	return strings.TrimSpace(cleaned.String())
}

// isSourceContextLine checks if a line is Bun's source code context.
// These lines have the format: "1234 | <code>" where 1234 is a line number.
func isSourceContextLine(line string) bool {
	// Find the pipe separator
	pipeIdx := strings.Index(line, "|")
	if pipeIdx < 1 {
		return false
	}

	// Check if everything before the pipe is digits and whitespace
	prefix := strings.TrimSpace(line[:pipeIdx])
	if prefix == "" {
		return false
	}

	for _, ch := range prefix {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	return true
}
