package errors

import (
	"errors"
	"fmt"
)

// ClaudeSDKError is the base interface for all SDK errors.
type ClaudeSDKError interface {
	error
	IsClaudeSDKError() bool
}

// Compile-time verification that all error types implement ClaudeSDKError.
var (
	_ ClaudeSDKError = (*CLINotFoundError)(nil)
	_ ClaudeSDKError = (*CLIConnectionError)(nil)
	_ ClaudeSDKError = (*ProcessError)(nil)
	_ ClaudeSDKError = (*MessageParseError)(nil)
	_ ClaudeSDKError = (*CLIJSONDecodeError)(nil)
)

// Sentinel errors for commonly checked conditions.
var (
	// ErrClientNotConnected indicates the client is not connected.
	ErrClientNotConnected = errors.New("client not connected")

	// ErrClientAlreadyConnected indicates the client is already connected.
	ErrClientAlreadyConnected = errors.New("client already connected")

	// ErrClientClosed indicates the client has been closed and cannot be reused.
	ErrClientClosed = errors.New("client closed: clients are single-use, create a new one with New()")

	// ErrTransportNotConnected indicates the transport is not connected.
	ErrTransportNotConnected = errors.New("transport not connected")

	// ErrRequestTimeout indicates a request timed out.
	ErrRequestTimeout = errors.New("request timeout")

	// ErrControllerStopped indicates the protocol controller has stopped.
	ErrControllerStopped = errors.New("protocol controller stopped")

	// ErrStdinClosed indicates stdin was closed due to context cancellation.
	ErrStdinClosed = errors.New("stdin closed")

	// ErrOperationCancelled indicates an operation was cancelled via cancel request.
	ErrOperationCancelled = errors.New("operation cancelled")

	// ErrUnknownMessageType indicates the message type is not recognized by the SDK.
	// Callers should skip these messages rather than treating them as fatal.
	ErrUnknownMessageType = errors.New("unknown message type")

	// ErrSessionNotFound indicates no persisted session was found for the given session ID.
	ErrSessionNotFound = errors.New("session not found")
)

// CLINotFoundError indicates the Claude CLI binary was not found.
type CLINotFoundError struct {
	SearchedPaths []string
}

func (e *CLINotFoundError) Error() string {
	return fmt.Sprintf("claude CLI not found in: %v", e.SearchedPaths)
}

// IsClaudeSDKError implements ClaudeSDKError.
func (e *CLINotFoundError) IsClaudeSDKError() bool { return true }

// CLIConnectionError indicates failure to connect to the CLI.
type CLIConnectionError struct {
	Err error
}

func (e *CLIConnectionError) Error() string {
	return fmt.Sprintf("failed to connect to CLI: %v", e.Err)
}

func (e *CLIConnectionError) Unwrap() error {
	return e.Err
}

// IsClaudeSDKError implements ClaudeSDKError.
func (e *CLIConnectionError) IsClaudeSDKError() bool { return true }

// ProcessError indicates the CLI process failed.
type ProcessError struct {
	ExitCode int
	Stderr   string
	Err      error
}

func (e *ProcessError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("CLI process failed (exit %d): %v", e.ExitCode, e.Err)
	}

	return fmt.Sprintf("CLI process failed (exit %d): %s", e.ExitCode, e.Stderr)
}

func (e *ProcessError) Unwrap() error {
	return e.Err
}

// IsClaudeSDKError implements ClaudeSDKError.
func (e *ProcessError) IsClaudeSDKError() bool { return true }

// MessageParseError indicates message parsing failed.
type MessageParseError struct {
	Message string
	Err     error
	Data    map[string]any
}

func (e *MessageParseError) Error() string {
	return fmt.Sprintf("failed to parse message: %v", e.Err)
}

func (e *MessageParseError) Unwrap() error {
	return e.Err
}

// IsClaudeSDKError implements ClaudeSDKError.
func (e *MessageParseError) IsClaudeSDKError() bool { return true }

// CLIJSONDecodeError indicates JSON parsing failed for CLI output.
// This error preserves the original raw data that failed to parse.
type CLIJSONDecodeError struct {
	RawData string
	Err     error
}

func (e *CLIJSONDecodeError) Error() string {
	return fmt.Sprintf("failed to decode JSON from CLI: %v", e.Err)
}

func (e *CLIJSONDecodeError) Unwrap() error {
	return e.Err
}

// IsClaudeSDKError implements ClaudeSDKError.
func (e *CLIJSONDecodeError) IsClaudeSDKError() bool { return true }
