package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLINotFoundError(t *testing.T) {
	err := &CLINotFoundError{
		SearchedPaths: []string{"/usr/bin/claude", "/opt/bin/claude"},
	}

	require.Equal(
		t,
		"claude CLI not found in: [/usr/bin/claude /opt/bin/claude]",
		err.Error(),
	)
	require.True(t, err.IsClaudeSDKError())
}

func TestCLIConnectionError(t *testing.T) {
	root := errors.New("dial failed")
	err := &CLIConnectionError{Err: root}

	require.Equal(t, "failed to connect to CLI: dial failed", err.Error())
	require.ErrorIs(t, err, root)
	require.True(t, err.IsClaudeSDKError())
}

func TestProcessError_WithUnderlyingError(t *testing.T) {
	root := errors.New("process terminated")
	err := &ProcessError{
		ExitCode: 9,
		Stderr:   "ignored when Err is set",
		Err:      root,
	}

	require.Equal(t, "CLI process failed (exit 9): process terminated", err.Error())
	require.ErrorIs(t, err, root)
	require.True(t, err.IsClaudeSDKError())
}

func TestProcessError_WithStderrOnly(t *testing.T) {
	err := &ProcessError{
		ExitCode: 2,
		Stderr:   "permission denied",
	}

	require.Equal(t, "CLI process failed (exit 2): permission denied", err.Error())
	require.NoError(t, err.Unwrap())
	require.True(t, err.IsClaudeSDKError())
}

func TestMessageParseError(t *testing.T) {
	root := errors.New("bad payload")
	err := &MessageParseError{
		Message: "decode failed",
		Err:     root,
		Data: map[string]any{
			"type": "unknown",
		},
	}

	require.Equal(t, "failed to parse message: bad payload", err.Error())
	require.ErrorIs(t, err, root)
	require.True(t, err.IsClaudeSDKError())
}

func TestCLIJSONDecodeError(t *testing.T) {
	root := errors.New("unexpected token")
	err := &CLIJSONDecodeError{
		RawData: `{"not":"valid",`,
		Err:     root,
	}

	require.Equal(t, "failed to decode JSON from CLI: unexpected token", err.Error())
	require.ErrorIs(t, err, root)
	require.True(t, err.IsClaudeSDKError())
}
