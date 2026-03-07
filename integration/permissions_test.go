//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
	"github.com/stretchr/testify/require"
)

// TestToolPermissions_AllowExplicit tests CanUseTool returning PermissionResultAllow.
func TestToolPermissions_AllowExplicit(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "5")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tmpDir := t.TempDir()

	var toolCallCount int32

	for msg, err := range claudesdk.Query(ctx,
		"Use Bash to create a file named canuse_allow.txt in the current directory.",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithCwd(tmpDir),
		claudesdk.WithPermissionMode("default"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithCanUseTool(func(
			_ context.Context,
			toolName string,
			_ map[string]any,
			_ *claudesdk.ToolPermissionContext,
		) (claudesdk.PermissionResult, error) {
			atomic.AddInt32(&toolCallCount, 1)
			t.Logf("Tool permission check: %s", toolName)

			return &claudesdk.PermissionResultAllow{
				Behavior: "allow",
			}, nil
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			require.False(t, result.IsError, "Query should not result in error")
		}
	}

	toolCalls := atomic.LoadInt32(&toolCallCount)
	require.Greater(t, toolCalls, int32(0), "CanUseTool callback should have been invoked")
}

// TestToolPermissions_Deny tests CanUseTool returning PermissionResultDeny.
func TestToolPermissions_Deny(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "5")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "canuse_deny.txt")

	var deniedTool string

	for _, err := range claudesdk.Query(ctx,
		"Use Bash to create a file named canuse_deny.txt in the current directory.",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithCwd(tmpDir),
		claudesdk.WithPermissionMode("default"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithCanUseTool(func(
			_ context.Context,
			toolName string,
			_ map[string]any,
			_ *claudesdk.ToolPermissionContext,
		) (claudesdk.PermissionResult, error) {
			if deniedTool == "" {
				deniedTool = toolName
			}

			return &claudesdk.PermissionResultDeny{
				Behavior: "deny",
				Message:  "Tool use denied in integration test",
			}, nil
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}
	}

	require.NotEmpty(t, deniedTool, "At least one tool should have been denied via callback")
	_, err := os.Stat(targetFile)
	require.True(t, os.IsNotExist(err), "Denied command should not create target file")
}

// TestToolPermissions_ModifyInput tests modifying tool input via UpdatedInput.
func TestToolPermissions_ModifyInput(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "5")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "input_mod_test.txt")

	var inputModified bool

	for _, err := range claudesdk.Query(ctx,
		"Use Bash to run: echo original > input_mod_test.txt",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithCwd(tmpDir),
		claudesdk.WithPermissionMode("default"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithCanUseTool(func(
			_ context.Context,
			toolName string,
			input map[string]any,
			_ *claudesdk.ToolPermissionContext,
		) (claudesdk.PermissionResult, error) {
			if toolName == "Bash" {
				if cmd, ok := input["command"].(string); ok {
					modifiedInput := make(map[string]any, len(input))
					for k, v := range input {
						modifiedInput[k] = v
					}

					modifiedInput["command"] = strings.ReplaceAll(
						cmd,
						"echo original > input_mod_test.txt",
						"echo modified > input_mod_test.txt",
					)
					inputModified = true

					return &claudesdk.PermissionResultAllow{
						Behavior:     "allow",
						UpdatedInput: modifiedInput,
					}, nil
				}
			}

			return &claudesdk.PermissionResultAllow{
				Behavior: "allow",
			}, nil
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}
	}

	require.True(t, inputModified, "Tool input should have been modified")
	data, err := os.ReadFile(targetFile)
	require.NoError(t, err, "Modified command should create target file")
	require.Contains(t, string(data), "modified", "File content should reflect updated input")
}

// TestToolPermissions_ClientInteractive tests CanUseTool through the interactive Client API.
func TestToolPermissions_ClientInteractive(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "5")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "client_canuse.txt")
	callbackCalled := false

	client := claudesdk.NewClient()
	defer client.Close()

	err := client.Start(ctx,
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithCwd(tmpDir),
		claudesdk.WithPermissionMode("default"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithCanUseTool(func(
			_ context.Context,
			toolName string,
			_ map[string]any,
			_ *claudesdk.ToolPermissionContext,
		) (claudesdk.PermissionResult, error) {
			callbackCalled = true
			t.Logf("Interactive client tool permission check: %s", toolName)

			return &claudesdk.PermissionResultAllow{
				Behavior: "allow",
			}, nil
		}),
	)
	if err != nil {
		skipIfCLINotInstalled(t, err)
		t.Fatalf("Client start failed: %v", err)
	}

	err = client.Query(ctx, "Use Bash to create a file named client_canuse.txt in the current directory.")
	require.NoError(t, err)

	for msg, recvErr := range client.ReceiveResponse(ctx) {
		require.NoError(t, recvErr)

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			for _, block := range m.Content {
				switch b := block.(type) {
				case *claudesdk.TextBlock:
					t.Logf("assistant text: %s", b.Text)
				case *claudesdk.ToolUseBlock:
					t.Logf("assistant tool use: %s %#v", b.Name, b.Input)
				default:
					t.Logf("assistant block: %T", b)
				}
			}
		case *claudesdk.UserMessage:
			if m.Content.IsString() {
				t.Logf("user content: %s", m.Content.String())
				continue
			}

			for _, block := range m.Content.Blocks() {
				if toolResult, ok := block.(*claudesdk.ToolResultBlock); ok {
					var parts []string
					for _, resultBlock := range toolResult.Content {
						if textBlock, ok := resultBlock.(*claudesdk.TextBlock); ok {
							parts = append(parts, textBlock.Text)
						}
					}

					t.Logf(
						"user tool result: is_error=%v tool_use_id=%s content=%q",
						toolResult.IsError,
						toolResult.ToolUseID,
						strings.Join(parts, "\n"),
					)
					continue
				}

				t.Logf("user block: %T %#v", block, block)
			}
		case *claudesdk.ResultMessage:
			t.Logf("result: is_error=%v subtype=%s result=%v", m.IsError, m.Subtype, m.Result)
		default:
			t.Logf("message type: %T", msg)
		}
	}

	data, err := os.ReadFile(targetFile)
	require.NoError(t, err, "Interactive client should create target file")
	require.NotNil(t, data)
	require.True(t, callbackCalled, "Interactive client should invoke CanUseTool")
}

// TestMCPTools_PermissionEnforcement tests that disallowed_tools blocks MCP tool execution
// and allowed_tools permits it. Split into two subtests with single-tool objectives
// to avoid flakiness from the LLM wasting turns on conflicting goals.
func TestMCPTools_PermissionEnforcement(t *testing.T) {
	newTools := func(t *testing.T, echoExecuted, greetExecuted *bool) (claudesdk.Tool, claudesdk.Tool) {
		t.Helper()

		echoTool := claudesdk.NewTool(
			"echo",
			"Echo back the input text",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{
						"type":        "string",
						"description": "Text to echo",
					},
				},
				"required": []string{"text"},
			},
			func(_ context.Context, input map[string]any) (map[string]any, error) {
				*echoExecuted = true
				text, _ := input["text"].(string)
				t.Logf("Echo tool executed with: %s", text)

				return map[string]any{
					"echo": text,
				}, nil
			},
		)

		greetTool := claudesdk.NewTool(
			"greet",
			"Greet a person by name",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Name of the person to greet",
					},
				},
				"required": []string{"name"},
			},
			func(_ context.Context, input map[string]any) (map[string]any, error) {
				*greetExecuted = true
				name, _ := input["name"].(string)
				t.Logf("Greet tool executed with: %s", name)

				return map[string]any{
					"greeting": "Hello, " + name + "!",
				}, nil
			},
		)

		return echoTool, greetTool
	}

	t.Run("disallowed_tool_is_blocked", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		var echoExecuted, greetExecuted bool

		echoTool, greetTool := newTools(t, &echoExecuted, &greetExecuted)

		for _, err := range claudesdk.Query(ctx,
			"Use the echo tool to echo 'test'",
			claudesdk.WithModel("claude-haiku-4-5"),
			claudesdk.WithPermissionMode("bypassPermissions"),
			claudesdk.WithMaxTurns(5),
			claudesdk.WithSDKTools(echoTool, greetTool),
			claudesdk.WithDisallowedTools("mcp__sdk__echo"),
			claudesdk.WithAllowedTools("mcp__sdk__greet"),
		) {
			if err != nil {
				skipIfCLINotInstalled(t, err)
				t.Fatalf("Query failed: %v", err)
			}
		}

		require.False(t, echoExecuted, "Disallowed echo tool should NOT have been executed")
	})

	t.Run("allowed_tool_executes", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		var echoExecuted, greetExecuted bool

		echoTool, greetTool := newTools(t, &echoExecuted, &greetExecuted)

		for _, err := range claudesdk.Query(ctx,
			"Use the greet tool to greet 'Alice'",
			claudesdk.WithModel("claude-haiku-4-5"),
			claudesdk.WithPermissionMode("bypassPermissions"),
			claudesdk.WithMaxTurns(5),
			claudesdk.WithSDKTools(echoTool, greetTool),
			claudesdk.WithDisallowedTools("mcp__sdk__echo"),
			claudesdk.WithAllowedTools("mcp__sdk__greet"),
		) {
			if err != nil {
				skipIfCLINotInstalled(t, err)
				t.Fatalf("Query failed: %v", err)
			}
		}

		require.True(t, greetExecuted, "Allowed greet tool should have been executed")
		require.False(t, echoExecuted, "Echo tool should not have been executed")
	})
}
