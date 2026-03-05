package claudesdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
	sdkerrors "github.com/ethpandaops/claude-agent-sdk-go/internal/errors"
	internalmcp "github.com/ethpandaops/claude-agent-sdk-go/internal/mcp"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/message"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/protocol"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/subprocess"
)

const (
	// defaultStreamCloseTimeout is the default timeout for waiting for result before closing stdin.
	defaultStreamCloseTimeout = 60 * time.Second
)

// getStreamCloseTimeout returns the stream close timeout from env var or default.
func getStreamCloseTimeout() time.Duration {
	if timeoutStr := os.Getenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT"); timeoutStr != "" {
		if timeoutSec, err := strconv.Atoi(timeoutStr); err == nil && timeoutSec > 0 {
			return time.Duration(timeoutSec) * time.Second
		}
	}

	return defaultStreamCloseTimeout
}

// createStreamingTransport creates a transport for streaming mode.
func createStreamingTransport(
	log *slog.Logger,
	options *ClaudeAgentOptions,
) config.Transport {
	if options.Transport != nil {
		log.Debug("Using injected custom transport for streaming")

		return options.Transport
	}

	log.Debug("Creating CLI transport in streaming mode")

	return subprocess.NewCLITransportWithMode(log, "", options, true)
}

// getLoggerWithComponent returns a logger with the component field set.
func getLoggerWithComponent(options *ClaudeAgentOptions, component string) *slog.Logger {
	log := options.Logger
	if log == nil {
		log = NopLogger()
	}

	return log.With("component", component)
}

// validateAndConfigureOptions validates options and configures auto-settings.
// It returns an error if callback-based tool handling and PermissionPromptToolName
// are both set.
func validateAndConfigureOptions(options *ClaudeAgentOptions) error {
	if options.CanUseTool != nil || options.OnUserInput != nil {
		if options.PermissionPromptToolName != "" {
			if options.CanUseTool != nil {
				return fmt.Errorf(
					"can_use_tool callback cannot be used with permission_prompt_tool_name",
				)
			}

			return fmt.Errorf(
				"on_user_input callback cannot be used with permission_prompt_tool_name",
			)
		}

		options.PermissionPromptToolName = "stdio"
	}

	return nil
}

// hasSDKMCPServer returns true if options include at least one in-process SDK MCP server.
func hasSDKMCPServer(options *ClaudeAgentOptions) bool {
	if options == nil || len(options.MCPServers) == 0 {
		return false
	}

	for _, serverConfig := range options.MCPServers {
		if sdkConfig, ok := serverConfig.(*internalmcp.SdkServerConfig); ok && sdkConfig != nil {
			if _, ok := sdkConfig.Instance.(internalmcp.ServerInstance); ok {
				return true
			}
		}
	}

	return false
}

// queryRequiresStreamingMode returns true when Query needs bidirectional stdin.
// This is required for initialize/control callbacks used by hooks, can_use_tool,
// OnUserInput handling, in-process SDK MCP servers, and agent definitions.
func queryRequiresStreamingMode(options *ClaudeAgentOptions) bool {
	if options == nil {
		return false
	}

	return len(options.Hooks) > 0 ||
		options.CanUseTool != nil ||
		options.OnUserInput != nil ||
		len(options.Agents) > 0 ||
		hasSDKMCPServer(options)
}

// Query executes a one-shot query to Claude and returns an iterator of messages.
//
// By default, logging is disabled. Use WithLogger to enable logging:
//
//	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
//	for msg, err := range Query(ctx, "What is 2+2?",
//	    WithLogger(logger),
//	    WithPermissionMode("acceptEdits"),
//	) {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    // handle msg
//	}
//
// The iterator yields messages as they arrive from Claude, including assistant
// responses, tool use, and a final result message. Any errors during setup or
// execution are yielded inline with messages, allowing callers to handle all
// error conditions.
//
// Query supports hooks, CanUseTool callbacks, OnUserInput callbacks, and SDK MCP servers through
// the protocol controller. When these options are configured, an initialization
// request is sent to the CLI before processing messages.
//
// Example usage:
//
//	ctx := context.Background()
//	for msg, err := range Query(ctx, "What is 2+2?",
//	    WithPermissionMode("acceptEdits"),
//	    WithMaxTurns(1),
//	) {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    switch m := msg.(type) {
//	    case *AssistantMessage:
//	        // Handle assistant response
//	    case *ResultMessage:
//	        // Handle final result
//	    }
//	}
//
// Error Handling:
//
// Errors are yielded inline as the second return value. The iterator
// distinguishes between recoverable and fatal errors:
//
//   - Parse errors: If a message from Claude cannot be parsed, the error
//     is yielded and iteration continues with the next message. This allows
//     callers to log malformed messages without losing subsequent data.
//
//   - Fatal errors: Transport failures, context cancellation, and controller
//     errors cause iteration to stop after yielding the error.
//
// Callers can always stop iteration early by breaking from the loop,
// regardless of error type.
func Query(
	ctx context.Context,
	prompt string,
	opts ...Option,
) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		// Apply options
		options := applyAgentOptions(opts)

		// Validate and configure options
		if err := validateAndConfigureOptions(options); err != nil {
			yield(nil, err)

			return
		}

		// Query in --print mode can deadlock when initialize/callback traffic is required.
		// Route these cases through streaming mode while preserving the Query API.
		if queryRequiresStreamingMode(options) {
			stream := MessagesFromSlice([]StreamingMessage{NewUserMessage(prompt)})

			for msg, err := range QueryStream(ctx, stream, opts...) {
				if !yield(msg, err) {
					return
				}
			}

			return
		}

		// Use provided logger or silent logger
		log := options.Logger
		if log == nil {
			log = NopLogger()
		}

		log = log.With("component", "query")
		log.Debug("Starting query execution")

		// Create or use injected transport
		var transport config.Transport

		if options.Transport != nil {
			transport = options.Transport

			log.Debug("Using injected custom transport")
		} else {
			log.Debug("Creating CLI transport")

			transport = subprocess.NewCLITransport(log, prompt, options)
		}

		// Start the transport
		log.Info("Starting transport")

		if err := transport.Start(ctx); err != nil {
			log.Error("Failed to start CLI", "error", err)
			yield(nil, err)

			return
		}

		defer transport.Close()

		log.Info("Successfully started Claude CLI")

		// Create protocol controller for bidirectional communication
		controller := protocol.NewController(log, transport)
		if err := controller.Start(ctx); err != nil {
			yield(nil, fmt.Errorf("start protocol controller: %w", err))

			return
		}

		defer controller.Stop()

		// Create session for protocol handling
		session := protocol.NewSession(log, controller, options)
		session.RegisterMCPServers()
		session.RegisterHandlers()

		// Initialize if we have callbacks that need it
		if session.NeedsInitialization() {
			log.Debug("Initializing session for hooks/callbacks")

			if err := session.Initialize(ctx); err != nil {
				yield(nil, fmt.Errorf("initialize session: %w", err))

				return
			}
		}

		// Close stdin to signal the CLI that no more input is coming (one-shot mode)
		// The CLI in --print mode waits for stdin to close before processing
		log.Debug("Closing stdin for one-shot query mode")

		if err := transport.EndInput(); err != nil {
			yield(nil, fmt.Errorf("close stdin: %w", err))

			return
		}

		// Get message channel from controller (controller is the sole reader from transport)
		rawMessages := controller.Messages()

		log.Debug("Reading messages from controller")

		for {
			select {
			case msg, ok := <-rawMessages:
				if !ok {
					// Channel closed, check for fatal error
					log.Debug("Raw message channel closed")

					if err := controller.FatalError(); err != nil {
						log.Error("Error from transport", "error", err)
						yield(nil, err)
					}

					return
				}

				// Parse the message
				parsed, err := message.Parse(log, msg)
				if errors.Is(err, sdkerrors.ErrUnknownMessageType) {
					continue
				}

				if err != nil {
					log.Warn("Failed to parse message", "error", err)

					if !yield(nil, fmt.Errorf("parse message: %w", err)) {
						return
					}

					continue
				}

				// Yield parsed message
				if !yield(parsed, nil) {
					log.Debug("Yield returned false, stopping iteration")

					return
				}

			case <-controller.Done():
				// Controller stopped (possibly due to transport error)
				log.Debug("Controller stopped")

				if err := controller.FatalError(); err != nil {
					log.Error("Error from transport", "error", err)
					yield(nil, err)
				}

				return

			case <-ctx.Done():
				log.Debug("Context cancelled")
				yield(nil, ctx.Err())

				return
			}
		}
	}
}

// streamInputMessages streams messages to the transport's stdin.
// Returns error if sending fails, nil on successful completion.
func streamInputMessages(
	ctx context.Context,
	log *slog.Logger,
	transport config.Transport,
	messages iter.Seq[StreamingMessage],
	hasMCPOrHooks bool,
	resultReceived <-chan struct{},
	streamCloseTimeout time.Duration,
) (err error) {
	defer func() {
		if endErr := transport.EndInput(); endErr != nil {
			if err == nil {
				err = fmt.Errorf("end input: %w", endErr)
			}
		}
	}()

	for msg := range messages {
		select {
		case <-ctx.Done():
			log.Debug("Context cancelled during message streaming")

			return ctx.Err()
		default:
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Error("Failed to marshal streaming message", "error", err)

			return fmt.Errorf("marshal streaming message: %w", err)
		}

		if err := transport.SendMessage(ctx, data); err != nil {
			log.Error("Failed to send streaming message", "error", err)

			return fmt.Errorf("send streaming message: %w", err)
		}

		log.Debug("Sent streaming message to CLI")
	}

	log.Debug("Finished streaming all messages")

	if hasMCPOrHooks {
		log.Debug("Waiting for result before closing stdin (MCP/hooks present)")

		select {
		case <-resultReceived:
			log.Debug("Result received, proceeding to close stdin")
		case <-time.After(streamCloseTimeout):
			log.Warn("Timeout waiting for result before closing stdin", "timeout", streamCloseTimeout)
		case <-ctx.Done():
			log.Debug("Context cancelled while waiting for result")

			return ctx.Err()
		}
	}

	return nil
}

// QueryStream executes a streaming query with multiple input messages.
//
// The messages iterator yields StreamingMessage values that are sent to Claude
// via stdin in streaming mode. The transport uses --input-format stream-json.
//
// By default, logging is disabled. Use WithLogger to enable logging.
//
// The iterator yields messages as they arrive from Claude, including assistant
// responses, tool use, and a final result message. Any errors during setup or
// execution are yielded inline with messages, allowing callers to handle all
// error conditions.
//
// QueryStream supports hooks, CanUseTool callbacks, OnUserInput callbacks, and SDK MCP servers through
// the protocol controller. When these options are configured, an initialization
// request is sent to the CLI before processing messages.
//
// Example usage:
//
//	ctx := context.Background()
//	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
//
//	messages := claudesdk.MessagesFromSlice([]claudesdk.StreamingMessage{
//	    claudesdk.NewUserMessage("Hello"),
//	    claudesdk.NewUserMessage("How are you?"),
//	})
//
//	for msg, err := range claudesdk.QueryStream(ctx, messages,
//	    claudesdk.WithLogger(logger),
//	    claudesdk.WithPermissionMode("acceptEdits"),
//	) {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    // Handle messages
//	}
//
// Error Handling:
//
// Errors are yielded inline as the second return value. The iterator
// distinguishes between recoverable and fatal errors:
//
//   - Parse errors: If a message from Claude cannot be parsed, the error
//     is yielded and iteration continues with the next message. This allows
//     callers to log malformed messages without losing subsequent data.
//
//   - Fatal errors: Transport failures, context cancellation, and controller
//     errors cause iteration to stop after yielding the error.
//
// Callers can always stop iteration early by breaking from the loop,
// regardless of error type.
func QueryStream(
	ctx context.Context,
	messages iter.Seq[StreamingMessage],
	opts ...Option,
) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		// Apply options
		options := applyAgentOptions(opts)

		// Validate and configure options
		if err := validateAndConfigureOptions(options); err != nil {
			yield(nil, err)

			return
		}

		log := getLoggerWithComponent(options, "query_stream")
		log.Debug("Starting streaming query execution")

		// Create transport
		transport := createStreamingTransport(log, options)

		// Start the transport
		log.Info("Starting transport in streaming mode")

		if err := transport.Start(ctx); err != nil {
			log.Error("Failed to start CLI", "error", err)
			yield(nil, err)

			return
		}

		defer transport.Close()

		log.Info("Successfully started Claude CLI in streaming mode")

		// Create protocol controller for bidirectional communication
		controller := protocol.NewController(log, transport)
		if err := controller.Start(ctx); err != nil {
			yield(nil, fmt.Errorf("start protocol controller: %w", err))

			return
		}

		defer controller.Stop()

		// Create session for protocol handling
		session := protocol.NewSession(log, controller, options)
		session.RegisterMCPServers()
		session.RegisterHandlers()

		// Streaming mode always needs initialization
		log.Debug("Initializing session for streaming mode")

		if err := session.Initialize(ctx); err != nil {
			yield(nil, fmt.Errorf("initialize session: %w", err))

			return
		}

		// Get message channel from controller (controller is the sole reader from transport)
		rawMessages := controller.Messages()

		// If bidirectional callbacks may be used, wait for result before closing stdin.
		// This includes MCP, hooks, and tool callbacks.
		hasMCPOrHooks := len(options.MCPServers) > 0 ||
			len(options.Hooks) > 0 ||
			options.CanUseTool != nil ||
			options.OnUserInput != nil

		// Channel to signal when first result is received (only used when hasMCPOrHooks)
		var resultReceived chan struct{}

		if hasMCPOrHooks {
			resultReceived = make(chan struct{})
		}

		// Safely close resultReceived channel exactly once
		var closeResultOnce sync.Once

		closeResult := func() {
			if resultReceived != nil {
				closeResultOnce.Do(func() {
					close(resultReceived)
				})
			}
		}

		// Get stream close timeout from env var or use default
		streamCloseTimeout := getStreamCloseTimeout()

		// Create errgroup for goroutine management
		g, gCtx := errgroup.WithContext(ctx)

		// Start goroutine to stream messages to stdin
		g.Go(func() error {
			return streamInputMessages(
				gCtx,
				log,
				transport,
				messages,
				hasMCPOrHooks,
				resultReceived,
				streamCloseTimeout,
			)
		})

		// IMPORTANT: Defer order is LIFO - these execute in reverse order
		// 1. First close resultReceived to unblock streamInputMessages goroutine
		// 2. Then wait for errgroup goroutines to complete
		// Note: Error from g.Wait() is already yielded via gCtx.Done() case in select loop
		defer func() { _ = g.Wait() }()

		defer func() {
			if hasMCPOrHooks {
				closeResult()
			}
		}()

		log.Debug("Reading messages from controller")

		for {
			select {
			case msg, ok := <-rawMessages:
				if !ok {
					log.Debug("Raw message channel closed")

					// Check for fatal error from controller
					if err := controller.FatalError(); err != nil {
						log.Error("Error from transport", "error", err)
						yield(nil, err)
					}

					return
				}

				parsed, err := message.Parse(log, msg)
				if errors.Is(err, sdkerrors.ErrUnknownMessageType) {
					continue
				}

				if err != nil {
					log.Warn("Failed to parse message", "error", err)

					if !yield(nil, fmt.Errorf("parse message: %w", err)) {
						return
					}

					continue
				}

				if hasMCPOrHooks {
					if _, isResult := parsed.(*message.ResultMessage); isResult {
						closeResult()
					}
				}

				if !yield(parsed, nil) {
					log.Debug("Yield returned false, stopping iteration")

					return
				}

			case <-controller.Done():
				// Controller stopped (possibly due to transport error)
				log.Debug("Controller stopped")

				if err := controller.FatalError(); err != nil {
					log.Error("Error from transport", "error", err)
					yield(nil, err)
				}

				return

			case <-ctx.Done():
				log.Debug("Context cancelled")
				yield(nil, ctx.Err())

				return

			case <-gCtx.Done():
				// Errgroup context cancelled - streaming goroutine failed
				if err := g.Wait(); err != nil {
					log.Error("Streaming goroutine failed", "error", err)
					yield(nil, err)
				}

				return
			}
		}
	}
}
