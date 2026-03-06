package claudesdk

import (
	"context"
	"iter"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/client"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/message"
)

// clientWrapper wraps the internal client to adapt it to the public interface.
type clientWrapper struct {
	impl *client.Client
}

// Compile-time check that *clientWrapper implements the Client interface.
var _ Client = (*clientWrapper)(nil)

// newClientImpl creates the internal client implementation.
func newClientImpl() Client {
	return &clientWrapper{impl: client.New()}
}

// Start establishes a connection to the Claude CLI.
func (c *clientWrapper) Start(ctx context.Context, opts ...Option) error {
	return c.impl.Start(ctx, applyAgentOptionsToConfig(opts))
}

// StartWithPrompt establishes a connection and immediately sends an initial prompt.
func (c *clientWrapper) StartWithPrompt(ctx context.Context, prompt string, opts ...Option) error {
	return c.impl.StartWithPrompt(ctx, prompt, applyAgentOptionsToConfig(opts))
}

// StartWithStream establishes a connection and streams initial messages.
func (c *clientWrapper) StartWithStream(
	ctx context.Context,
	messages iter.Seq[StreamingMessage],
	opts ...Option,
) error {
	// Convert the public streaming type to the internal message type.
	convertedMessages := func(yield func(message.StreamingMessage) bool) {
		for msg := range messages {
			if !yield(msg) {
				return
			}
		}
	}

	return c.impl.StartWithStream(ctx, convertedMessages, applyAgentOptionsToConfig(opts))
}

// Query sends a user prompt to Claude.
func (c *clientWrapper) Query(ctx context.Context, prompt string, sessionID ...string) error {
	return c.impl.Query(ctx, prompt, sessionID...)
}

// ReceiveMessages returns an iterator that yields messages indefinitely.
func (c *clientWrapper) ReceiveMessages(ctx context.Context) iter.Seq2[Message, error] {
	return c.impl.ReceiveMessages(ctx)
}

// ReceiveResponse returns an iterator that yields messages until a ResultMessage is received.
func (c *clientWrapper) ReceiveResponse(ctx context.Context) iter.Seq2[Message, error] {
	return c.impl.ReceiveResponse(ctx)
}

// Interrupt sends an interrupt signal to stop Claude's current processing.
func (c *clientWrapper) Interrupt(ctx context.Context) error {
	return c.impl.Interrupt(ctx)
}

// SetPermissionMode changes the permission mode during conversation.
func (c *clientWrapper) SetPermissionMode(ctx context.Context, mode string) error {
	return c.impl.SetPermissionMode(ctx, mode)
}

// SetModel changes the AI model during conversation.
func (c *clientWrapper) SetModel(ctx context.Context, model *string) error {
	return c.impl.SetModel(ctx, model)
}

// ListModels returns the SDK's static Claude model catalog in the ListModels payload shape.
func (c *clientWrapper) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return c.impl.ListModels(ctx)
}

// GetServerInfo returns server initialization info including available commands.
func (c *clientWrapper) GetServerInfo() map[string]any {
	return c.impl.GetServerInfo()
}

// GetMCPStatus queries the CLI for live MCP server connection status.
func (c *clientWrapper) GetMCPStatus(ctx context.Context) (*MCPStatus, error) {
	return c.impl.GetMCPStatus(ctx)
}

// ReconnectMCPServer reconnects a disconnected or failed MCP server.
func (c *clientWrapper) ReconnectMCPServer(ctx context.Context, serverName string) error {
	return c.impl.ReconnectMCPServer(ctx, serverName)
}

// ToggleMCPServer enables or disables an MCP server.
func (c *clientWrapper) ToggleMCPServer(ctx context.Context, serverName string, enabled bool) error {
	return c.impl.ToggleMCPServer(ctx, serverName, enabled)
}

// StopTask stops a running task by task ID.
func (c *clientWrapper) StopTask(ctx context.Context, taskID string) error {
	return c.impl.StopTask(ctx, taskID)
}

// RewindFiles rewinds tracked files to their state at a specific user message.
func (c *clientWrapper) RewindFiles(ctx context.Context, userMessageID string) error {
	return c.impl.RewindFiles(ctx, userMessageID)
}

// SendToolResult sends a tool_result message back to Claude for a pending tool_use.
func (c *clientWrapper) SendToolResult(ctx context.Context, toolUseID, content string, isError bool) error {
	return c.impl.SendToolResult(ctx, toolUseID, content, isError)
}

// Close terminates the session and cleans up resources.
func (c *clientWrapper) Close() error {
	return c.impl.Close()
}

// applyAgentOptionsToConfig converts public options to internal config.Options.
func applyAgentOptionsToConfig(opts []Option) *config.Options {
	options := applyAgentOptions(opts)
	if options == nil {
		return nil
	}
	// The public options type shares the same underlying representation.
	return options
}
