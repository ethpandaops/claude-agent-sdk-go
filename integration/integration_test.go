//go:build integration

package integration

import (
	"errors"
	"strings"
	"testing"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

// skipIfCLINotInstalled skips the test if the error indicates the CLI is not found.
func skipIfCLINotInstalled(t *testing.T, err error) {
	t.Helper()

	if _, ok := errors.AsType[*claudesdk.CLINotFoundError](err); ok {
		t.Skip("Claude CLI not installed")
	}
}

// contains42 checks if a string contains "42" in various formats.
func contains42(s string) bool {
	lower := strings.ToLower(s)

	return strings.Contains(lower, "42") ||
		strings.Contains(lower, "forty-two") ||
		strings.Contains(lower, "forty two")
}
