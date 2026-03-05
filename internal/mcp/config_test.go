package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerConfigGetType(t *testing.T) {
	t.Run("stdio defaults to stdio type when nil", func(t *testing.T) {
		cfg := &StdioServerConfig{
			Command: "server-binary",
		}

		require.Equal(t, ServerTypeStdio, cfg.GetType())
	})

	t.Run("stdio uses explicit type when set", func(t *testing.T) {
		explicit := ServerTypeSSE
		cfg := &StdioServerConfig{
			Type:    &explicit,
			Command: "server-binary",
		}

		require.Equal(t, ServerTypeSSE, cfg.GetType())
	})

	t.Run("sse/http/sdk configs return their configured type", func(t *testing.T) {
		sse := &SSEServerConfig{Type: ServerTypeSSE}
		http := &HTTPServerConfig{Type: ServerTypeHTTP}
		sdk := &SdkServerConfig{Type: ServerTypeSDK}

		require.Equal(t, ServerTypeSSE, sse.GetType())
		require.Equal(t, ServerTypeHTTP, http.GetType())
		require.Equal(t, ServerTypeSDK, sdk.GetType())
	})
}
