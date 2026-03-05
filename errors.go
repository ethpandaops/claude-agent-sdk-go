package claudesdk

import "github.com/ethpandaops/claude-agent-sdk-go/internal/errors"

// Re-export error types from internal package

// CLINotFoundError indicates the Claude CLI binary was not found.
type CLINotFoundError = errors.CLINotFoundError

// CLIConnectionError indicates failure to connect to the CLI.
type CLIConnectionError = errors.CLIConnectionError

// ProcessError indicates the CLI process failed.
type ProcessError = errors.ProcessError

// MessageParseError indicates message parsing failed.
type MessageParseError = errors.MessageParseError

// CLIJSONDecodeError indicates JSON parsing failed for CLI output.
type CLIJSONDecodeError = errors.CLIJSONDecodeError

// ClaudeSDKError is the base interface for all SDK errors.
type ClaudeSDKError = errors.ClaudeSDKError

// Re-export sentinel errors from internal package.
var (
	// ErrClientNotConnected indicates the client is not connected.
	ErrClientNotConnected = errors.ErrClientNotConnected

	// ErrClientAlreadyConnected indicates the client is already connected.
	ErrClientAlreadyConnected = errors.ErrClientAlreadyConnected

	// ErrClientClosed indicates the client has been closed and cannot be reused.
	ErrClientClosed = errors.ErrClientClosed

	// ErrTransportNotConnected indicates the transport is not connected.
	ErrTransportNotConnected = errors.ErrTransportNotConnected

	// ErrRequestTimeout indicates a request timed out.
	ErrRequestTimeout = errors.ErrRequestTimeout

	// ErrSessionNotFound indicates no persisted session was found for the given session ID.
	ErrSessionNotFound = errors.ErrSessionNotFound
)
