//go:build integration

package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

// TestHooks_PreToolUse tests hook invoked before tool execution.
func TestHooks_PreToolUse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var hookInvoked int32

	timeout := 30.0

	for _, err := range claudesdk.Query(ctx, "List files in the current directory using ls",
		claudesdk.WithModel("haiku"),
		claudesdk.WithPermissionMode("acceptAll"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPreToolUse: {{
				Hooks: []claudesdk.HookCallback{
					func(_ context.Context, input claudesdk.HookInput,
						_ *string, _ *claudesdk.HookContext,
					) (claudesdk.HookJSONOutput, error) {
						atomic.AddInt32(&hookInvoked, 1)

						if preInput, ok := input.(*claudesdk.PreToolUseHookInput); ok {
							t.Logf("PreToolUse hook called for tool: %s", preInput.ToolName)
						}

						continueFlag := true

						return &claudesdk.SyncHookJSONOutput{
							Continue: &continueFlag,
						}, nil
					},
				},
				Timeout: &timeout,
			}},
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}
	}

	require.Greater(t, atomic.LoadInt32(&hookInvoked), int32(0),
		"PreToolUse hook should have been invoked")
}

// TestHooks_PostToolUse tests hook invoked after tool execution.
func TestHooks_PostToolUse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var hookInvoked int32

	timeout := 30.0

	for _, err := range claudesdk.Query(ctx, "Run 'echo hello' command",
		claudesdk.WithModel("haiku"),
		claudesdk.WithPermissionMode("acceptAll"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPostToolUse: {{
				Hooks: []claudesdk.HookCallback{
					func(_ context.Context, input claudesdk.HookInput,
						_ *string, _ *claudesdk.HookContext,
					) (claudesdk.HookJSONOutput, error) {
						atomic.AddInt32(&hookInvoked, 1)

						if postInput, ok := input.(*claudesdk.PostToolUseHookInput); ok {
							t.Logf("PostToolUse hook called for tool: %s", postInput.ToolName)
						}

						continueFlag := true

						return &claudesdk.SyncHookJSONOutput{
							Continue: &continueFlag,
						}, nil
					},
				},
				Timeout: &timeout,
			}},
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}
	}

	require.Greater(t, atomic.LoadInt32(&hookInvoked), int32(0),
		"PostToolUse hook should have been invoked")
}

// TestHooks_BlockTool tests PreToolUse with Continue: false blocks tool.
func TestHooks_BlockTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var toolBlocked int32

	bashTool := "Bash"
	timeout := 30.0

	for _, err := range claudesdk.Query(ctx, "Run 'echo blocked' command",
		claudesdk.WithModel("haiku"),
		claudesdk.WithPermissionMode("acceptAll"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPreToolUse: {{
				Matcher: &bashTool,
				Hooks: []claudesdk.HookCallback{
					func(_ context.Context, _ claudesdk.HookInput,
						_ *string, _ *claudesdk.HookContext,
					) (claudesdk.HookJSONOutput, error) {
						atomic.AddInt32(&toolBlocked, 1)
						t.Logf("Blocking Bash tool")

						continueFlag := false
						denyDecision := "deny"
						reason := "Tool blocked by test hook"

						return &claudesdk.SyncHookJSONOutput{
							Continue: &continueFlag,
							HookSpecificOutput: &claudesdk.PreToolUseHookSpecificOutput{
								PermissionDecision:       &denyDecision,
								PermissionDecisionReason: &reason,
							},
						}, nil
					},
				},
				Timeout: &timeout,
			}},
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}
	}

	require.Greater(t, atomic.LoadInt32(&toolBlocked), int32(0),
		"Bash tool should have been blocked by hook")
}

// TestHooks_WithAdditionalContext tests PostToolUse hook with additionalContext field.
func TestHooks_WithAdditionalContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var (
		hookInvoked       int32
		receivedToolInput map[string]any
	)

	timeout := 30.0

	for _, err := range claudesdk.Query(ctx, "Run 'echo test' command",
		claudesdk.WithModel("haiku"),
		claudesdk.WithPermissionMode("acceptAll"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPostToolUse: {{
				Hooks: []claudesdk.HookCallback{
					func(_ context.Context, input claudesdk.HookInput,
						_ *string, _ *claudesdk.HookContext,
					) (claudesdk.HookJSONOutput, error) {
						atomic.AddInt32(&hookInvoked, 1)

						if postInput, ok := input.(*claudesdk.PostToolUseHookInput); ok {
							receivedToolInput = postInput.ToolInput
							t.Logf("PostToolUse hook for tool: %s, input: %v",
								postInput.ToolName, postInput.ToolInput)
						}

						continueFlag := true
						additionalContext := "This is additional context from the hook"

						return &claudesdk.SyncHookJSONOutput{
							Continue: &continueFlag,
							HookSpecificOutput: &claudesdk.PostToolUseHookSpecificOutput{
								AdditionalContext: &additionalContext,
							},
						}, nil
					},
				},
				Timeout: &timeout,
			}},
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}
	}

	require.Greater(t, atomic.LoadInt32(&hookInvoked), int32(0),
		"PostToolUse hook should have been invoked")
	require.NotNil(t, receivedToolInput, "Hook should have received tool input")
}
