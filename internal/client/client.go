package client

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/errors"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/mcp"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/message"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/models"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/protocol"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/subprocess"
)

const (
	// defaultMessageBufferSize is the buffer size for the messages channel.
	defaultMessageBufferSize = 10

	// interruptTimeout is the timeout for interrupt control requests.
	interruptTimeout = 5 * time.Second

	// rewindFilesTimeout is the timeout for rewind_files control requests.
	rewindFilesTimeout = 10 * time.Second

	// setPermissionModeTimeout is the timeout for set_permission_mode control requests.
	setPermissionModeTimeout = 5 * time.Second

	// setModelTimeout is the timeout for set_model control requests.
	setModelTimeout = 5 * time.Second

	// mcpStatusTimeout is the timeout for mcp_status control requests.
	mcpStatusTimeout = 10 * time.Second

	// mcpControlTimeout is the timeout for MCP control requests.
	mcpControlTimeout = 10 * time.Second

	// stopTaskTimeout is the timeout for stop_task control requests.
	stopTaskTimeout = 10 * time.Second
)

// Client implements the interactive client interface.
type Client struct {
	log        *slog.Logger
	transport  config.Transport
	controller *protocol.Controller
	session    *protocol.Session
	options    *config.Options

	// Message channel for data flow
	messages chan message.Message

	// Fatal error storage (replaces error channel)
	errMu    sync.RWMutex
	fatalErr error

	// Errgroup for goroutine management
	eg *errgroup.Group

	// Lifecycle management
	mu        sync.Mutex
	done      chan struct{}
	connected bool
	closed    bool      // Tracks if Close() has been called
	closeOnce sync.Once // Ensures Close() only runs once
}

// New creates a new interactive client.
//
// The client is not connected after creation. Call Start() with options to connect.
func New() *Client {
	return &Client{
		messages: make(chan message.Message, defaultMessageBufferSize),
		done:     make(chan struct{}),
	}
}

// setFatalError stores the first fatal error encountered.
func (c *Client) setFatalError(err error) {
	if err == nil {
		return
	}

	c.errMu.Lock()
	defer c.errMu.Unlock()

	if c.fatalErr == nil {
		c.fatalErr = err
	}
}

// getFatalError returns the stored fatal error, if any.
func (c *Client) getFatalError() error {
	c.errMu.RLock()
	defer c.errMu.RUnlock()

	return c.fatalErr
}

// isConnected returns true if the client is connected.
// This method is safe to call from any goroutine.
func (c *Client) isConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.connected
}

// initializeCore performs common client initialization.
// Caller must hold c.mu lock. Lock is held on return.
func (c *Client) initializeCore(ctx context.Context, options *config.Options) error {
	// Default to empty options if nil
	if options == nil {
		options = &config.Options{}
	}

	// Extract logger from options, defaulting to a no-op logger
	log := options.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	c.log = log.With("component", "client")

	// When callback-based tool handling is set, automatically configure CLI to
	// send permission requests via control protocol.
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

	// Store options for callback handlers
	c.options = options

	// Create or use injected transport
	var transport config.Transport

	if options.Transport != nil {
		transport = options.Transport

		c.log.Debug("Using injected custom transport")
	} else {
		// For interactive sessions, use streaming mode (isStreaming=true) for control protocol
		transport = subprocess.NewCLITransportWithMode(c.log, "", options, true)
	}

	if err := transport.Start(ctx); err != nil {
		return fmt.Errorf("start transport: %w", err)
	}

	c.transport = transport

	// Create protocol controller for bidirectional communication
	c.controller = protocol.NewController(c.log, transport)
	if err := c.controller.Start(ctx); err != nil {
		transport.Close()

		return fmt.Errorf("start protocol controller: %w", err)
	}

	// Create session for protocol handling
	c.session = protocol.NewSession(c.log, c.controller, options)
	c.session.RegisterMCPServers()
	c.session.RegisterHandlers()

	// Send initialization request to tell CLI about hooks and get server info
	if err := c.session.Initialize(ctx); err != nil {
		transport.Close()

		return fmt.Errorf("initialize session: %w", err)
	}

	return nil
}

// Start establishes a connection to the Claude CLI.
//
// This method spawns the CLI subprocess and sets up bidirectional communication.
// For interactive sessions, no initial prompt is sent - use Query() to send prompts.
//
// Returns CLINotFoundError if the CLI binary cannot be located,
// or CLIConnectionError if the process fails to start.
func (c *Client) Start(ctx context.Context, options *config.Options) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.ErrClientClosed
	}

	if c.connected {
		return errors.ErrClientAlreadyConnected
	}

	if err := c.initializeCore(ctx, options); err != nil {
		return err
	}

	c.log.Info("Starting transport")

	// Create errgroup with background context for goroutine management.
	// We use context.Background() instead of the caller's ctx because:
	// 1. The caller's ctx may have a timeout for initialization operations
	// 2. When that timeout expires, it would kill readLoop() and streamMessages()
	// 3. The client should remain connected until explicitly closed via Close()
	// 4. The c.done channel provides explicit shutdown signaling
	var egCtx context.Context

	c.eg, egCtx = errgroup.WithContext(context.Background())

	// Start read loop using errgroup
	c.eg.Go(func() error {
		return c.readLoop(egCtx)
	})

	c.connected = true
	c.log.Info("Client started successfully")

	return nil
}

// StartWithPrompt establishes a connection and immediately sends an initial prompt.
//
// This is a convenience method equivalent to calling Start() followed by Query().
// The prompt is sent to the "default" session.
func (c *Client) StartWithPrompt(
	ctx context.Context,
	prompt string,
	options *config.Options,
) error {
	if err := c.Start(ctx, options); err != nil {
		return err
	}

	return c.Query(ctx, prompt)
}

// StartWithStream establishes a connection and streams initial messages.
//
// This method starts the client in streaming mode and consumes messages from the
// provided iterator. Messages are sent to the CLI via stdin. The iterator runs
// in a separate goroutine; use context cancellation to abort message streaming.
// EndInput is called automatically when the iterator completes.
func (c *Client) StartWithStream(
	ctx context.Context,
	messages iter.Seq[message.StreamingMessage],
	options *config.Options,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.ErrClientClosed
	}

	if c.connected {
		return errors.ErrClientAlreadyConnected
	}

	if err := c.initializeCore(ctx, options); err != nil {
		return err
	}

	c.log.Info("Starting transport in streaming mode")

	// Create errgroup with background context for goroutine management.
	// We use context.Background() instead of the caller's ctx because:
	// 1. The caller's ctx may have a timeout for initialization operations
	// 2. When that timeout expires, it would kill readLoop() and streamMessages()
	// 3. The client should remain connected until explicitly closed via Close()
	// 4. The c.done channel provides explicit shutdown signaling
	var egCtx context.Context

	c.eg, egCtx = errgroup.WithContext(context.Background())

	// Start goroutine to stream messages to stdin
	c.eg.Go(func() error {
		return c.streamMessages(egCtx, messages)
	})

	// Start read loop using errgroup
	c.eg.Go(func() error {
		return c.readLoop(egCtx)
	})

	c.connected = true
	c.log.Info("Client started successfully in streaming mode")

	return nil
}

// streamMessages sends streaming messages to the transport.
// Returns error if sending fails, nil on successful completion.
func (c *Client) streamMessages(
	ctx context.Context,
	messages iter.Seq[message.StreamingMessage],
) (err error) {
	defer func() {
		if endErr := c.transport.EndInput(); endErr != nil {
			if err == nil {
				err = fmt.Errorf("end input: %w", endErr)
			}
		}
	}()

	for msg := range messages {
		select {
		case <-ctx.Done():
			c.log.Debug("Context cancelled during message streaming")

			return ctx.Err()
		case <-c.done:
			c.log.Debug("Client closed during message streaming")

			return nil
		default:
		}

		data, err := json.Marshal(msg)
		if err != nil {
			c.log.Error("Failed to marshal streaming message", "error", err)

			return fmt.Errorf("marshal streaming message: %w", err)
		}

		if err := c.transport.SendMessage(ctx, data); err != nil {
			c.log.Error("Failed to send streaming message", "error", err)

			return fmt.Errorf("send streaming message: %w", err)
		}

		c.log.Debug("Sent streaming message to CLI")
	}

	c.log.Debug("Finished streaming all messages")

	return nil
}

// readLoop reads messages from the controller and routes them to the messages channel.
// The controller is the sole reader from transport - it filters control messages and forwards
// regular messages through its Messages() channel.
// Returns error if a fatal error occurs, nil on normal completion.
func (c *Client) readLoop(ctx context.Context) error {
	defer c.log.Debug("Read loop stopped")
	defer close(c.messages)

	// Use controller's filtered message channel (controller is the sole reader from transport)
	rawMessages := c.controller.Messages()

	for {
		select {
		case msg, ok := <-rawMessages:
			if !ok {
				c.log.Debug("Message channel closed")

				// Check for fatal error from controller
				if err := c.controller.FatalError(); err != nil {
					c.log.Error("Transport error", "error", err)
					c.setFatalError(err)

					return err
				}

				return nil
			}

			// Control messages are already filtered by the controller
			parsed, err := message.Parse(c.log, msg)
			if stderrors.Is(err, errors.ErrUnknownMessageType) {
				continue
			}

			if err != nil {
				c.log.Warn("Failed to parse message", "error", err)
				c.setFatalError(fmt.Errorf("parse message: %w", err))

				return fmt.Errorf("parse message: %w", err)
			}

			select {
			case c.messages <- parsed:
			case <-c.done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}

		case <-c.controller.Done():
			c.log.Debug("Controller stopped")

			// Forward fatal error if present
			if err := c.controller.FatalError(); err != nil {
				c.log.Error("Transport error", "error", err)
				c.setFatalError(err)

				return err
			}

			return nil

		case <-c.done:
			return nil

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Query sends a user prompt to Claude.
//
// This method sends a user_message to the CLI and returns immediately.
// Use Receive() to get the responses.
// Optional sessionID defaults to "default" for multi-session support.
func (c *Client) Query(ctx context.Context, prompt string, sessionID ...string) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	// Default to "default" session
	sid := "default"
	if len(sessionID) > 0 && sessionID[0] != "" {
		sid = sessionID[0]
	}

	c.log.Debug("Sending query", "prompt_len", len(prompt), "session_id", sid)

	// Send user_message via transport
	payload := map[string]any{
		"type":               "user",
		"message":            map[string]any{"role": "user", "content": prompt},
		"parent_tool_use_id": nil,
		"session_id":         sid,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal query: %w", err)
	}

	return c.transport.SendMessage(ctx, data)
}

// receive waits for and returns the next message from Claude.
//
// This method blocks until a message is available, an error occurs, or the
// context is cancelled. Returns io.EOF when the session ends normally.
// This is an internal method used by ReceiveMessages and ReceiveResponse.
func (c *Client) receive(ctx context.Context) (message.Message, error) {
	// Check for stored fatal error first
	if err := c.getFatalError(); err != nil {
		return nil, err
	}

	select {
	case msg, ok := <-c.messages:
		if !ok {
			// Channel closed, wait for errgroup and check for errors
			if c.eg != nil {
				if err := c.eg.Wait(); err != nil {
					c.setFatalError(err)

					return nil, err
				}
			}

			return nil, io.EOF
		}

		return msg, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReceiveMessages returns an iterator that yields messages indefinitely.
// Messages are yielded as they arrive until EOF, an error occurs, or context is cancelled.
// Unlike ReceiveResponse, this iterator does not stop at ResultMessage.
func (c *Client) ReceiveMessages(ctx context.Context) iter.Seq2[message.Message, error] {
	return func(yield func(message.Message, error) bool) {
		if !c.isConnected() {
			yield(nil, errors.ErrClientNotConnected)

			return
		}

		for {
			msg, err := c.receive(ctx)
			if err != nil {
				yield(nil, err)

				return
			}

			if !yield(msg, nil) {
				return
			}
		}
	}
}

// Interrupt sends an interrupt signal to stop Claude's current processing.
//
// This uses the protocol controller to send a control_request with subtype "interrupt".
func (c *Client) Interrupt(ctx context.Context) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	c.log.Info("Sending interrupt signal")

	_, err := c.controller.SendRequest(ctx, "interrupt", nil, interruptTimeout)
	if err != nil {
		return fmt.Errorf("send interrupt signal: %w", err)
	}

	return nil
}

// RewindFiles rewinds tracked files to their state at a specific user message.
//
// The userMessageID should be the ID of a previous user message in the conversation.
// The CLI must support file checkpointing for this to work.
func (c *Client) RewindFiles(ctx context.Context, userMessageID string) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	c.log.Info("Rewinding files", "user_message_id", userMessageID)

	payload := map[string]any{
		"user_message_id": userMessageID,
	}

	_, err := c.controller.SendRequest(ctx, "rewind_files", payload, rewindFilesTimeout)
	if err != nil {
		return fmt.Errorf("rewind files: %w", err)
	}

	return nil
}

// SetPermissionMode changes the permission mode during conversation.
// Valid modes: "default", "acceptEdits", "plan", "bypassPermissions".
func (c *Client) SetPermissionMode(ctx context.Context, mode string) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	normalizedMode := config.NormalizePermissionMode(mode)

	c.log.Info("Setting permission mode", "mode", normalizedMode)

	payload := map[string]any{
		"mode": normalizedMode,
	}

	_, err := c.controller.SendRequest(ctx, "set_permission_mode", payload, setPermissionModeTimeout)
	if err != nil {
		return fmt.Errorf("set permission mode to %q: %w", normalizedMode, err)
	}

	return nil
}

// SetModel changes the AI model during conversation.
//
// Pass nil to use the default model.
func (c *Client) SetModel(ctx context.Context, model *string) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	c.log.Info("Setting model", "model", model)

	payload := map[string]any{
		"model": model,
	}

	_, err := c.controller.SendRequest(ctx, "set_model", payload, setModelTimeout)
	if err != nil {
		return fmt.Errorf("set model: %w", err)
	}

	return nil
}

// ListModels returns the SDK's static Claude model catalog in the ListModels payload shape.
func (c *Client) ListModels(_ context.Context) ([]models.Info, error) {
	return models.List(), nil
}

// GetMCPStatus queries the CLI for live MCP server connection status.
// Returns the status of all configured MCP servers.
func (c *Client) GetMCPStatus(ctx context.Context) (*mcp.Status, error) {
	if !c.isConnected() {
		return nil, errors.ErrClientNotConnected
	}

	c.log.Info("Querying MCP server status")

	resp, err := c.controller.SendRequest(ctx, "mcp_status", nil, mcpStatusTimeout)
	if err != nil {
		return nil, fmt.Errorf("get mcp status: %w", err)
	}

	payload := resp.Payload()

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal mcp status payload: %w", err)
	}

	var status mcp.Status
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("unmarshal mcp status: %w", err)
	}

	// Append in-process SDK MCP servers that the CLI doesn't know about.
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()

	if session != nil {
		for _, name := range session.GetSDKMCPServerNames() {
			server, ok := session.GetSDKMCPServer(name)
			if !ok {
				continue
			}

			tools := server.ListTools()
			toolInfos := make([]mcp.ToolInfo, 0, len(tools))

			for _, tool := range tools {
				info := mcp.ToolInfo{}
				if toolName, ok := tool["name"].(string); ok {
					info.Name = toolName
				}

				if description, ok := tool["description"].(string); ok {
					info.Description = description
				}

				if annotations, ok := tool["annotations"].(map[string]any); ok {
					toolAnnotations := &mcp.ToolAnnotations{}
					if v, ok := annotations["readOnly"].(bool); ok {
						toolAnnotations.ReadOnly = v
					}

					if v, ok := annotations["destructive"].(bool); ok {
						toolAnnotations.Destructive = v
					}

					if v, ok := annotations["openWorld"].(bool); ok {
						toolAnnotations.OpenWorld = v
					}

					info.Annotations = toolAnnotations
				}

				toolInfos = append(toolInfos, info)
			}

			status.MCPServers = append(status.MCPServers, mcp.ServerStatus{
				Name:   name,
				Status: mcp.ServerConnectionStatusConnected,
				ServerInfo: &mcp.ServerInfo{
					Name:    server.Name(),
					Version: server.Version(),
				},
				Config: &mcp.ServerStatusConfig{
					Type: string(mcp.ServerTypeSDK),
					Name: name,
				},
				Tools: toolInfos,
			})
		}
	}

	return &status, nil
}

// ReconnectMCPServer reconnects a disconnected or failed MCP server.
func (c *Client) ReconnectMCPServer(ctx context.Context, serverName string) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	payload := map[string]any{
		"serverName": serverName,
	}

	_, err := c.controller.SendRequest(ctx, "mcp_reconnect", payload, mcpControlTimeout)
	if err != nil {
		return fmt.Errorf("reconnect mcp server %q: %w", serverName, err)
	}

	return nil
}

// ToggleMCPServer enables or disables an MCP server.
func (c *Client) ToggleMCPServer(ctx context.Context, serverName string, enabled bool) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	payload := map[string]any{
		"serverName": serverName,
		"enabled":    enabled,
	}

	_, err := c.controller.SendRequest(ctx, "mcp_toggle", payload, mcpControlTimeout)
	if err != nil {
		return fmt.Errorf("toggle mcp server %q: %w", serverName, err)
	}

	return nil
}

// StopTask stops a running task by task ID.
func (c *Client) StopTask(ctx context.Context, taskID string) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	payload := map[string]any{
		"task_id": taskID,
	}

	_, err := c.controller.SendRequest(ctx, "stop_task", payload, stopTaskTimeout)
	if err != nil {
		return fmt.Errorf("stop task %q: %w", taskID, err)
	}

	return nil
}

// GetServerInfo returns server initialization info including available commands.
// Returns nil if not connected or not in streaming mode.
func (c *Client) GetServerInfo() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return nil
	}

	return c.session.GetInitializationResult()
}

// SendToolResult sends a tool_result message back to Claude for a pending
// tool_use. This allows the SDK to intercept tool calls (e.g. AskUserQuestion),
// gather a result externally, and feed it back so Claude continues processing.
func (c *Client) SendToolResult(ctx context.Context, toolUseID, content string, isError bool) error {
	if !c.isConnected() {
		return errors.ErrClientNotConnected
	}

	c.log.Debug("Sending tool result",
		"tool_use_id", toolUseID,
		"is_error", isError,
	)

	payload := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": toolUseID,
				"content":     content,
				"is_error":    isError,
			}},
		},
		"parent_tool_use_id": nil,
		"session_id":         "default",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal tool result: %w", err)
	}

	return c.transport.SendMessage(ctx, data)
}

// ReceiveResponse returns an iterator that yields messages until a ResultMessage is received.
// Messages are yielded as they arrive for streaming consumption.
// The iterator stops after yielding the ResultMessage.
func (c *Client) ReceiveResponse(ctx context.Context) iter.Seq2[message.Message, error] {
	return func(yield func(message.Message, error) bool) {
		if !c.isConnected() {
			yield(nil, errors.ErrClientNotConnected)

			return
		}

		for {
			msg, err := c.receive(ctx)
			if err != nil {
				yield(nil, fmt.Errorf("receive response: %w", err))

				return
			}

			// Yield the message; stop if consumer requests
			if !yield(msg, nil) {
				return
			}

			// Stop after ResultMessage
			if _, ok := msg.(*message.ResultMessage); ok {
				return
			}
		}
	}
}

// Close terminates the session and cleans up resources.
//
// After Close(), the client cannot be reused - create a new client with New().
// This method is safe to call multiple times.
func (c *Client) Close() error {
	var closeErr error

	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		wasConnected := c.connected
		c.connected = false
		c.mu.Unlock()

		if !wasConnected {
			return
		}

		c.log.Info("Closing client")

		// Signal shutdown
		close(c.done)

		// Stop protocol controller
		if c.controller != nil {
			c.controller.Stop()
		}

		// Close transport and capture error
		if c.transport != nil {
			closeErr = c.transport.Close()
		}

		// Wait for errgroup goroutines to complete
		if c.eg != nil {
			if err := c.eg.Wait(); err != nil && closeErr == nil {
				closeErr = err
			}
		}

		c.log.Info("Client closed")
	})

	return closeErr
}
