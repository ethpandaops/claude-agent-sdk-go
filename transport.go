package claudesdk

import "github.com/ethpandaops/claude-agent-sdk-go/internal/config"

// Transport defines the interface for Claude CLI communication.
// Implement this to provide custom transports for testing, mocking,
// or alternative communication methods (e.g., remote connections).
//
// The default implementation is CLITransport which spawns a subprocess.
// Custom transports can be injected via ClaudeAgentOptions.Transport.
type Transport = config.Transport
