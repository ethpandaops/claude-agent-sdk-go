//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
	"github.com/stretchr/testify/require"
)

// TestQueryIntegration tests end-to-end query execution.
func TestQueryIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	receivedMessages := 0
	receivedResult := false

	for msg, err := range claudesdk.Query(ctx, "What is 2+2? Reply with just the number.",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("acceptEdits"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		receivedMessages++

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			t.Logf("Received assistant message with %d content blocks", len(m.Content))
			for _, block := range m.Content {
				if text, ok := block.(*claudesdk.TextBlock); ok {
					t.Logf("Text: %s", text.Text)
				}
			}

		case *claudesdk.ResultMessage:
			t.Logf("Received result: duration=%dms turns=%d error=%v",
				m.DurationMs, m.NumTurns, m.IsError)
			receivedResult = true
			require.False(t, m.IsError, "Query should not result in error")

		case *claudesdk.SystemMessage:
			t.Logf("Received system message: %s", m.Subtype)

		case *claudesdk.UserMessage:
			t.Logf("Received user message with %d content blocks", len(m.Content.Blocks()))
		}
	}

	require.Greater(t, receivedMessages, 0, "Should receive at least one message")
	require.True(t, receivedResult, "Should receive result message")
}

// TestQueryWithLoggerIntegration tests query with explicit logger via options.
func TestQueryWithLoggerIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messageCount := 0

	for msg, err := range claudesdk.Query(ctx, "Say 'hello'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("acceptEdits"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		_ = msg
		messageCount++
	}

	require.Greater(t, messageCount, 0, "Should receive messages")
}

// TestQueryContextTimeout tests that query respects context timeout.
func TestQueryContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	for _, err := range claudesdk.Query(ctx, "This is a test",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("acceptEdits"),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Logf("Query failed (expected with short timeout): %v", err)

			return
		}
	}
}

// TestQueryErrorHandling tests error handling scenarios.
func TestQueryErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		cliPath string
		wantErr bool
	}{
		{
			name:    "invalid CLI path",
			cliPath: "/nonexistent/path/to/claude",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			for _, err := range claudesdk.Query(ctx, "test",
				claudesdk.WithCliPath(tt.cliPath),
			) {
				if tt.wantErr {
					require.Error(t, err)
					require.True(t, func() bool {
						_, ok := errors.AsType[*claudesdk.CLINotFoundError](err)
						return ok
					}())
				} else {
					if err != nil && !func() bool {
						_, ok := errors.AsType[*claudesdk.CLINotFoundError](err)
						return ok
					}() {
						t.Fatalf("Unexpected error: %v", err)
					}
				}

				break
			}
		})
	}
}

// TestQuery_ContinuationOption tests conversation continuation using ContinueConversation.
func TestQuery_ContinuationOption(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, err := range claudesdk.Query(ctx, "Remember the number 42.",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("First query failed: %v", err)
		}
	}

	receivedResult := false

	for msg, err := range claudesdk.Query(ctx, "What number did I ask you to remember?",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
		claudesdk.WithContinueConversation(true),
	) {
		if err != nil {
			t.Fatalf("Continuation query failed: %v", err)
		}

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			receivedResult = true
			require.False(t, result.IsError, "Continuation query should not result in error")
		}
	}

	require.True(t, receivedResult, "Should receive result message from continuation")
}

// TestQuery_MaxBudgetUsdOption tests budget limiting behavior.
func TestQuery_MaxBudgetUsdOption(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	maxBudget := 0.10

	receivedResult := false

	for msg, err := range claudesdk.Query(ctx, "Say 'budget test'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
		claudesdk.WithMaxBudgetUSD(maxBudget),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query with budget limit failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.ResultMessage:
			receivedResult = true
			if m.TotalCostUSD != nil {
				t.Logf("Query completed: cost_usd=%.6f", *m.TotalCostUSD)
			} else {
				t.Logf("Query completed: cost_usd=<nil>")
			}
			require.False(t, m.IsError, "Budget-limited query should succeed")
		}
	}

	require.True(t, receivedResult, "Should receive result message")
}

// TestQuery_WithToolUse tests query triggering tool execution.
func TestQuery_WithToolUse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var (
		receivedAssistant bool
		receivedResult    bool
		sawToolUse        bool
	)

	for msg, err := range claudesdk.Query(ctx, "Run the command 'echo hello world' using Bash",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(3),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query with tool use failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			receivedAssistant = true

			for _, block := range m.Content {
				if _, ok := block.(*claudesdk.ToolUseBlock); ok {
					sawToolUse = true
					t.Log("Saw tool use in assistant message")
				}
			}
		case *claudesdk.ResultMessage:
			receivedResult = true
			require.False(t, m.IsError, "Query should not result in error")
		}
	}

	require.True(t, receivedAssistant, "Should receive assistant message")
	require.True(t, receivedResult, "Should receive result message")

	t.Logf("Tool use observed: %v", sawToolUse)
}

// TestQuery_WithAllowedAndDisallowedTools tests allowed and disallowed tools configuration.
func TestQuery_WithAllowedAndDisallowedTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	receivedResult := false

	for msg, err := range claudesdk.Query(ctx, "Say 'tools configured'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
		claudesdk.WithAllowedTools("Read", "Grep"),
		claudesdk.WithDisallowedTools("Bash"),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query with tool configuration failed: %v", err)
		}

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			receivedResult = true
			require.False(t, result.IsError, "Query should not result in error")
		}
	}

	require.True(t, receivedResult, "Should receive result message")
}

// TestQuery_WithSettingSources tests different setting source configurations.
func TestQuery_WithSettingSources(t *testing.T) {
	tests := []struct {
		name    string
		sources []claudesdk.SettingSource
	}{
		{
			name:    "user only",
			sources: []claudesdk.SettingSource{claudesdk.SettingSourceUser},
		},
		{
			name:    "user and project",
			sources: []claudesdk.SettingSource{claudesdk.SettingSourceUser, claudesdk.SettingSourceProject},
		},
		{
			name:    "no sources (isolated)",
			sources: []claudesdk.SettingSource{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			receivedResult := false

			for msg, err := range claudesdk.Query(ctx, "Say 'settings test'",
				claudesdk.WithModel("claude-haiku-4-5"),
				claudesdk.WithPermissionMode("bypassPermissions"),
				claudesdk.WithMaxTurns(1),
				claudesdk.WithSettingSources(tt.sources...),
			) {
				if err != nil {
					skipIfCLINotInstalled(t, err)
					t.Fatalf("Query with setting sources failed: %v", err)
				}

				if result, ok := msg.(*claudesdk.ResultMessage); ok {
					receivedResult = true
					require.False(t, result.IsError, "Query should not result in error")
				}
			}

			require.True(t, receivedResult, "Should receive result message")
		})
	}
}
