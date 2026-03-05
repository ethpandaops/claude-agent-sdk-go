package claudesdk

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPermissionCallback_Allow tests permission callback allowing tool use.
func TestPermissionCallback_Allow(t *testing.T) {
	callback := func(
		_ context.Context,
		_ string,
		_ map[string]any,
		_ *ToolPermissionContext,
	) (PermissionResult, error) {
		return &PermissionResultAllow{Behavior: "allow"}, nil
	}

	ctx := context.Background()
	input := map[string]any{"command": "ls"}
	permCtx := &ToolPermissionContext{}

	result, err := callback(ctx, "Bash", input, permCtx)

	require.NoError(t, err)
	require.Equal(t, "allow", result.GetBehavior())
}

// TestPermissionCallback_Deny tests permission callback denying tool use.
func TestPermissionCallback_Deny(t *testing.T) {
	callback := func(
		_ context.Context,
		toolName string,
		input map[string]any,
		_ *ToolPermissionContext,
	) (PermissionResult, error) {
		if toolName == "Bash" {
			cmd, _ := input["command"].(string)
			if cmd == "rm -rf /" {
				return &PermissionResultDeny{
					Behavior: "deny",
					Message:  "Dangerous command blocked",
				}, nil
			}
		}

		return &PermissionResultAllow{Behavior: "allow"}, nil
	}

	ctx := context.Background()
	permCtx := &ToolPermissionContext{}

	dangerousInput := map[string]any{"command": "rm -rf /"}
	result, err := callback(ctx, "Bash", dangerousInput, permCtx)

	require.NoError(t, err)
	require.Equal(t, "deny", result.GetBehavior())

	denyResult, ok := result.(*PermissionResultDeny)
	require.True(t, ok)
	require.Equal(t, "Dangerous command blocked", denyResult.Message)
}

// TestPermissionCallback_InputModification tests callback that modifies input.
func TestPermissionCallback_InputModification(t *testing.T) {
	callback := func(
		_ context.Context,
		_ string,
		input map[string]any,
		_ *ToolPermissionContext,
	) (PermissionResult, error) {
		updatedInput := make(map[string]any, len(input)+1)
		maps.Copy(updatedInput, input)

		updatedInput["safe_mode"] = true

		return &PermissionResultAllow{
			Behavior:     "allow",
			UpdatedInput: updatedInput,
		}, nil
	}

	ctx := context.Background()
	input := map[string]any{"command": "test"}
	permCtx := &ToolPermissionContext{}

	result, err := callback(ctx, "Bash", input, permCtx)

	require.NoError(t, err)
	require.Equal(t, "allow", result.GetBehavior())

	allowResult, ok := result.(*PermissionResultAllow)
	require.True(t, ok)
	require.NotNil(t, allowResult.UpdatedInput)
	require.Equal(t, true, allowResult.UpdatedInput["safe_mode"])
	require.Equal(t, "test", allowResult.UpdatedInput["command"])
}

// TestPermissionCallback_ExceptionHandling tests callback exception handling.
func TestPermissionCallback_ExceptionHandling(t *testing.T) {
	callback := func(
		_ context.Context,
		_ string,
		_ map[string]any,
		_ *ToolPermissionContext,
	) (PermissionResult, error) {
		return nil, fmt.Errorf("callback error: database unavailable")
	}

	ctx := context.Background()
	input := map[string]any{}
	permCtx := &ToolPermissionContext{}

	result, err := callback(ctx, "Bash", input, permCtx)

	require.Error(t, err)
	require.Contains(t, err.Error(), "database unavailable")
	require.Nil(t, result)
}

// TestHookExecution tests basic hook execution.
func TestHookExecution(t *testing.T) {
	hookCalled := false

	hook := func(_ context.Context, _ HookInput, _ *string, _ *HookContext) (HookJSONOutput, error) {
		hookCalled = true

		return &SyncHookJSONOutput{}, nil
	}

	ctx := context.Background()
	input := &PreToolUseHookInput{
		BaseInput: BaseHookInput{SessionID: "test"},
		ToolName:  "Bash",
	}

	_, err := hook(ctx, input, nil, nil)

	require.NoError(t, err)
	require.True(t, hookCalled)
}

// TestHookOutputFields tests hook output field handling.
func TestHookOutputFields(t *testing.T) {
	continueVal := true
	decision := "allow"
	reason := "Approved by policy"
	systemMessage := "Operation authorized"

	output := &SyncHookJSONOutput{
		Continue:      &continueVal,
		Decision:      &decision,
		Reason:        &reason,
		SystemMessage: &systemMessage,
	}

	require.NotNil(t, output.Continue)
	require.True(t, *output.Continue)
	require.NotNil(t, output.Decision)
	require.Equal(t, "allow", *output.Decision)
	require.NotNil(t, output.Reason)
	require.Equal(t, "Approved by policy", *output.Reason)
	require.NotNil(t, output.SystemMessage)
	require.Equal(t, "Operation authorized", *output.SystemMessage)
}

// TestAsyncHookOutput tests async hook output.
func TestAsyncHookOutput(t *testing.T) {
	timeout := 60000
	output := &AsyncHookJSONOutput{
		AsyncTimeout: &timeout,
	}

	require.NotNil(t, output.AsyncTimeout)
	require.Equal(t, 60000, *output.AsyncTimeout)
}

// TestFieldNameConversion tests field name conversion.
func TestFieldNameConversion(t *testing.T) {
	// Test that async_ fields map to Go Async prefix
	asyncTimeout := 5000
	asyncOutput := &AsyncHookJSONOutput{
		AsyncTimeout: &asyncTimeout,
	}
	require.Equal(t, 5000, *asyncOutput.AsyncTimeout)

	// Test that continue_ fields work (Go uses Continue)
	continueVal := true
	syncOutput := &SyncHookJSONOutput{
		Continue: &continueVal,
	}
	require.True(t, *syncOutput.Continue)
}

// TestOptionsWithCallbacks tests options with permission callbacks.
func TestOptionsWithCallbacks(t *testing.T) {
	callbackCalled := false

	options := &ClaudeAgentOptions{
		CanUseTool: func(_ context.Context, _ string, _ map[string]any, _ *ToolPermissionContext) (PermissionResult, error) {
			callbackCalled = true

			return &PermissionResultAllow{Behavior: "allow"}, nil
		},
	}

	require.NotNil(t, options.CanUseTool)

	// Call the callback to verify it works
	ctx := context.Background()
	result, err := options.CanUseTool(ctx, "Bash", nil, nil)

	require.NoError(t, err)
	require.True(t, callbackCalled)
	require.Equal(t, "allow", result.GetBehavior())
}

// TestPermissionCallback_WithSuggestions tests permission callback receiving CLI suggestions.
func TestPermissionCallback_WithSuggestions(t *testing.T) {
	callback := func(
		_ context.Context,
		toolName string,
		input map[string]any,
		permCtx *ToolPermissionContext,
	) (PermissionResult, error) {
		// Verify suggestions are accessible
		if permCtx != nil && len(permCtx.Suggestions) > 0 {
			// Apply the first suggestion to the result
			return &PermissionResultAllow{
				Behavior:           "allow",
				UpdatedPermissions: permCtx.Suggestions,
			}, nil
		}

		return &PermissionResultAllow{Behavior: "allow"}, nil
	}

	ctx := context.Background()
	input := map[string]any{"command": "ls"}

	// Create permission context with suggestions from CLI
	behavior := PermissionBehaviorAllow
	permCtx := &ToolPermissionContext{
		Suggestions: []*PermissionUpdate{
			{
				Type: PermissionUpdateTypeAddRules,
				Rules: []*PermissionRuleValue{
					{
						ToolName:    "Bash",
						RuleContent: new("ls *"),
					},
				},
				Behavior: &behavior,
			},
		},
	}

	result, err := callback(ctx, "Bash", input, permCtx)

	require.NoError(t, err)
	require.Equal(t, "allow", result.GetBehavior())

	allowResult, ok := result.(*PermissionResultAllow)
	require.True(t, ok)
	require.NotNil(t, allowResult.UpdatedPermissions)
	require.Len(t, allowResult.UpdatedPermissions, 1)
	require.Equal(t, PermissionUpdateTypeAddRules, allowResult.UpdatedPermissions[0].Type)
}

// TestPermissionCallback_WithUpdatedPermissions tests permission callback returning updated permissions.
func TestPermissionCallback_WithUpdatedPermissions(t *testing.T) {
	callback := func(
		_ context.Context,
		toolName string,
		_ map[string]any,
		_ *ToolPermissionContext,
	) (PermissionResult, error) {
		behavior := PermissionBehaviorAllow
		dest := PermissionUpdateDestSession

		return &PermissionResultAllow{
			Behavior: "allow",
			UpdatedPermissions: []*PermissionUpdate{
				{
					Type: PermissionUpdateTypeAddRules,
					Rules: []*PermissionRuleValue{
						{
							ToolName:    toolName,
							RuleContent: new("echo *"),
						},
					},
					Behavior:    &behavior,
					Destination: &dest,
				},
			},
		}, nil
	}

	ctx := context.Background()
	result, err := callback(ctx, "Bash", nil, nil)

	require.NoError(t, err)

	allowResult, ok := result.(*PermissionResultAllow)
	require.True(t, ok)
	require.NotNil(t, allowResult.UpdatedPermissions)
	require.Len(t, allowResult.UpdatedPermissions, 1)

	update := allowResult.UpdatedPermissions[0]
	require.Equal(t, PermissionUpdateTypeAddRules, update.Type)
	require.Len(t, update.Rules, 1)
	require.Equal(t, "Bash", update.Rules[0].ToolName)
	require.NotNil(t, update.Destination)
	require.Equal(t, PermissionUpdateDestSession, *update.Destination)
}

// TestPermissionCallback_DenyWithInterrupt tests permission callback denying with interrupt.
func TestPermissionCallback_DenyWithInterrupt(t *testing.T) {
	callback := func(
		_ context.Context,
		_ string,
		_ map[string]any,
		_ *ToolPermissionContext,
	) (PermissionResult, error) {
		return &PermissionResultDeny{
			Behavior:  "deny",
			Message:   "Critical security violation",
			Interrupt: true,
		}, nil
	}

	ctx := context.Background()
	result, err := callback(ctx, "Bash", nil, nil)

	require.NoError(t, err)
	require.Equal(t, "deny", result.GetBehavior())

	denyResult, ok := result.(*PermissionResultDeny)
	require.True(t, ok)
	require.Equal(t, "Critical security violation", denyResult.Message)
	require.True(t, denyResult.Interrupt)
}

// TestPermissionUpdateToDict tests PermissionUpdate.ToDict conversion.
func TestPermissionUpdateToDict(t *testing.T) {
	behavior := PermissionBehaviorAllow
	dest := PermissionUpdateDestSession

	update := &PermissionUpdate{
		Type: PermissionUpdateTypeAddRules,
		Rules: []*PermissionRuleValue{
			{
				ToolName:    "Bash",
				RuleContent: new("echo *"),
			},
			{
				ToolName: "Read",
			},
		},
		Behavior:    &behavior,
		Destination: &dest,
	}

	dict := update.ToDict()

	require.Equal(t, "addRules", dict["type"])
	require.Equal(t, "session", dict["destination"])
	require.Equal(t, "allow", dict["behavior"])

	rules, ok := dict["rules"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, rules, 2)
	require.Equal(t, "Bash", rules[0]["toolName"])
	require.Equal(t, "echo *", rules[0]["ruleContent"])
	require.Equal(t, "Read", rules[1]["toolName"])
}

// TestMatcherMatching tests hook matcher functionality.
func TestMatcherMatching(t *testing.T) {
	bashTool := "Bash"
	matcher := &HookMatcher{
		Matcher: &bashTool,
	}

	require.NotNil(t, matcher.Matcher)
	require.Equal(t, "Bash", *matcher.Matcher)
}

// TestNewHookInputTypes tests construction and field access for new hook input types.
func TestNewHookInputTypes(t *testing.T) {
	t.Run("PostToolUseFailureInput", func(t *testing.T) {
		isInterrupt := true
		input := &PostToolUseFailureHookInput{
			BaseInput:     BaseHookInput{SessionID: "s1", Cwd: "/tmp"},
			HookEventName: "PostToolUseFailure",
			ToolName:      "Bash",
			ToolInput:     map[string]any{"command": "bad"},
			ToolUseID:     "tu_123",
			Error:         "command failed",
			IsInterrupt:   &isInterrupt,
		}

		require.Equal(t, HookEventPostToolUseFailure, input.GetHookEventName())
		require.Equal(t, "s1", input.GetSessionID())
		require.Equal(t, "Bash", input.ToolName)
		require.Equal(t, "tu_123", input.ToolUseID)
		require.Equal(t, "command failed", input.Error)
		require.NotNil(t, input.IsInterrupt)
		require.True(t, *input.IsInterrupt)
	})

	t.Run("NotificationInput", func(t *testing.T) {
		title := "Alert"
		input := &NotificationHookInput{
			BaseInput:        BaseHookInput{SessionID: "s2"},
			HookEventName:    "Notification",
			Message:          "Something happened",
			Title:            &title,
			NotificationType: "info",
		}

		require.Equal(t, HookEventNotification, input.GetHookEventName())
		require.Equal(t, "Something happened", input.Message)
		require.NotNil(t, input.Title)
		require.Equal(t, "Alert", *input.Title)
		require.Equal(t, "info", input.NotificationType)
	})

	t.Run("SubagentStartInput", func(t *testing.T) {
		input := &SubagentStartHookInput{
			BaseInput:     BaseHookInput{SessionID: "s3"},
			HookEventName: "SubagentStart",
			AgentID:       "agent_1",
			AgentType:     "Explore",
		}

		require.Equal(t, HookEventSubagentStart, input.GetHookEventName())
		require.Equal(t, "agent_1", input.AgentID)
		require.Equal(t, "Explore", input.AgentType)
	})

	t.Run("PermissionRequestInput", func(t *testing.T) {
		input := &PermissionRequestHookInput{
			BaseInput:             BaseHookInput{SessionID: "s4"},
			HookEventName:         "PermissionRequest",
			ToolName:              "Write",
			ToolInput:             map[string]any{"path": "/etc/hosts"},
			PermissionSuggestions: []any{"allow", "deny"},
		}

		require.Equal(t, HookEventPermissionRequest, input.GetHookEventName())
		require.Equal(t, "Write", input.ToolName)
		require.Len(t, input.PermissionSuggestions, 2)
	})

	t.Run("enhanced PreToolUseInput has ToolUseID", func(t *testing.T) {
		input := &PreToolUseHookInput{
			BaseInput: BaseHookInput{SessionID: "s5"},
			ToolName:  "Bash",
			ToolUseID: "tu_456",
		}

		require.Equal(t, "tu_456", input.ToolUseID)
	})

	t.Run("enhanced PostToolUseInput has ToolUseID", func(t *testing.T) {
		input := &PostToolUseHookInput{
			BaseInput: BaseHookInput{SessionID: "s6"},
			ToolName:  "Bash",
			ToolUseID: "tu_789",
		}

		require.Equal(t, "tu_789", input.ToolUseID)
	})

	t.Run("enhanced SubagentStopInput has agent fields", func(t *testing.T) {
		input := &SubagentStopHookInput{
			BaseInput:           BaseHookInput{SessionID: "s7"},
			AgentID:             "agent_2",
			AgentTranscriptPath: "/tmp/transcript.json",
			AgentType:           "Plan",
		}

		require.Equal(t, "agent_2", input.AgentID)
		require.Equal(t, "/tmp/transcript.json", input.AgentTranscriptPath)
		require.Equal(t, "Plan", input.AgentType)
	})
}

// TestNewHookSpecificOutputJSONSerialization tests JSON serialization of new specific output types.
func TestNewHookSpecificOutputJSONSerialization(t *testing.T) {
	t.Run("PostToolUseFailureSpecificOutput", func(t *testing.T) {
		ctx := new("Extra context about the failure")
		output := &PostToolUseFailureHookSpecificOutput{
			HookEventName:     "PostToolUseFailure",
			AdditionalContext: ctx,
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		jsonStr := string(data)
		require.Contains(t, jsonStr, `"hookEventName":"PostToolUseFailure"`)
		require.Contains(t, jsonStr, `"additionalContext":"Extra context about the failure"`)
	})

	t.Run("PostToolUseFailureSpecificOutput omitempty", func(t *testing.T) {
		output := &PostToolUseFailureHookSpecificOutput{
			HookEventName: "PostToolUseFailure",
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)
		require.NotContains(t, string(data), `"additionalContext"`)
	})

	t.Run("NotificationSpecificOutput", func(t *testing.T) {
		ctx := new("Notification context")
		output := &NotificationHookSpecificOutput{
			HookEventName:     "Notification",
			AdditionalContext: ctx,
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		jsonStr := string(data)
		require.Contains(t, jsonStr, `"hookEventName":"Notification"`)
		require.Contains(t, jsonStr, `"additionalContext":"Notification context"`)
	})

	t.Run("SubagentStartSpecificOutput", func(t *testing.T) {
		ctx := new("Subagent context")
		output := &SubagentStartHookSpecificOutput{
			HookEventName:     "SubagentStart",
			AdditionalContext: ctx,
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		jsonStr := string(data)
		require.Contains(t, jsonStr, `"hookEventName":"SubagentStart"`)
		require.Contains(t, jsonStr, `"additionalContext":"Subagent context"`)
	})

	t.Run("PermissionRequestSpecificOutput", func(t *testing.T) {
		output := &PermissionRequestHookSpecificOutput{
			HookEventName: "PermissionRequest",
			Decision:      map[string]any{"action": "allow", "reason": "trusted"},
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		jsonStr := string(data)
		require.Contains(t, jsonStr, `"hookEventName":"PermissionRequest"`)
		require.Contains(t, jsonStr, `"decision"`)
		require.Contains(t, jsonStr, `"action":"allow"`)
	})

	t.Run("PermissionRequestSpecificOutput omitempty", func(t *testing.T) {
		output := &PermissionRequestHookSpecificOutput{
			HookEventName: "PermissionRequest",
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)
		require.NotContains(t, string(data), `"decision"`)
	})

	t.Run("PreToolUseSpecificOutput has additionalContext", func(t *testing.T) {
		ctx := new("Pre-tool context")
		output := &PreToolUseHookSpecificOutput{
			HookEventName:     "PreToolUse",
			AdditionalContext: ctx,
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)
		require.Contains(t, string(data), `"additionalContext":"Pre-tool context"`)
	})

	t.Run("PostToolUseSpecificOutput has updatedMCPToolOutput", func(t *testing.T) {
		output := &PostToolUseHookSpecificOutput{
			HookEventName:        "PostToolUse",
			UpdatedMCPToolOutput: map[string]any{"result": "modified"},
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)
		require.Contains(t, string(data), `"updatedMCPToolOutput"`)
	})
}

// TestNewHookEventCallbackExecution tests hook callback execution with new event types.
func TestNewHookEventCallbackExecution(t *testing.T) {
	tests := []struct {
		name  string
		input HookInput
		event HookEvent
	}{
		{
			name: "PostToolUseFailure",
			input: &PostToolUseFailureHookInput{
				BaseInput: BaseHookInput{SessionID: "test"},
				ToolName:  "Bash",
				Error:     "failed",
			},
			event: HookEventPostToolUseFailure,
		},
		{
			name: "Notification",
			input: &NotificationHookInput{
				BaseInput:        BaseHookInput{SessionID: "test"},
				Message:          "hello",
				NotificationType: "info",
			},
			event: HookEventNotification,
		},
		{
			name: "SubagentStart",
			input: &SubagentStartHookInput{
				BaseInput: BaseHookInput{SessionID: "test"},
				AgentID:   "agent_1",
				AgentType: "Explore",
			},
			event: HookEventSubagentStart,
		},
		{
			name: "PermissionRequest",
			input: &PermissionRequestHookInput{
				BaseInput: BaseHookInput{SessionID: "test"},
				ToolName:  "Write",
			},
			event: HookEventPermissionRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedEvent HookEvent

			hookFn := func(
				_ context.Context,
				input HookInput,
				_ *string,
				_ *HookContext,
			) (HookJSONOutput, error) {
				receivedEvent = input.GetHookEventName()

				return &SyncHookJSONOutput{}, nil
			}

			ctx := context.Background()

			_, err := hookFn(ctx, tt.input, nil, nil)

			require.NoError(t, err)
			require.Equal(t, tt.event, receivedEvent)
		})
	}
}

// TestMultipleHooksExecuteInOrder tests multiple hooks execute in order.
func TestMultipleHooksExecuteInOrder(t *testing.T) {
	var order []int

	hook1 := func(_ context.Context, _ HookInput, _ *string, _ *HookContext) (HookJSONOutput, error) {
		order = append(order, 1)

		return &SyncHookJSONOutput{}, nil
	}

	hook2 := func(_ context.Context, _ HookInput, _ *string, _ *HookContext) (HookJSONOutput, error) {
		order = append(order, 2)

		return &SyncHookJSONOutput{}, nil
	}

	ctx := context.Background()
	input := &PreToolUseHookInput{
		BaseInput: BaseHookInput{SessionID: "test"},
		ToolName:  "Bash",
	}

	// Execute hooks in order
	_, _ = hook1(ctx, input, nil, nil)
	_, _ = hook2(ctx, input, nil, nil)

	require.Equal(t, []int{1, 2}, order)
}

// TestHookErrorPropagates tests hook error propagation.
func TestHookErrorPropagates(t *testing.T) {
	hook := func(_ context.Context, _ HookInput, _ *string, _ *HookContext) (HookJSONOutput, error) {
		return nil, fmt.Errorf("hook execution failed")
	}

	ctx := context.Background()
	input := &PreToolUseHookInput{
		BaseInput: BaseHookInput{SessionID: "test"},
		ToolName:  "Bash",
	}

	result, err := hook(ctx, input, nil, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "hook execution failed")
	require.Nil(t, result)
}

// TestEventDataPassedToHook tests event data is passed to hook correctly.
func TestEventDataPassedToHook(t *testing.T) {
	var (
		receivedToolName string
		receivedInput    map[string]any
	)

	hook := func(_ context.Context, input HookInput, _ *string, _ *HookContext) (HookJSONOutput, error) {
		if preInput, ok := input.(*PreToolUseHookInput); ok {
			receivedToolName = preInput.ToolName
			receivedInput = preInput.ToolInput
		}

		return &SyncHookJSONOutput{}, nil
	}

	ctx := context.Background()
	input := &PreToolUseHookInput{
		BaseInput: BaseHookInput{SessionID: "test-session"},
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "ls -la"},
	}

	_, err := hook(ctx, input, nil, nil)

	require.NoError(t, err)
	require.Equal(t, "Bash", receivedToolName)
	require.Equal(t, "ls -la", receivedInput["command"])
}

// TestNoHooksRegistered tests behavior when no hooks are registered.
func TestNoHooksRegistered(t *testing.T) {
	options := &ClaudeAgentOptions{
		Hooks: nil,
	}

	require.Nil(t, options.Hooks)

	// Empty hooks map should also work
	options.Hooks = make(map[HookEvent][]*HookMatcher)
	require.NotNil(t, options.Hooks)
	require.Empty(t, options.Hooks)
}

// TestHookOutputJSONSerialization tests that hook output types serialize to JSON with correct field names.
func TestHookOutputJSONSerialization(t *testing.T) {
	t.Run("async hook output has correct json field names", func(t *testing.T) {
		timeout := 5000
		output := &AsyncHookJSONOutput{
			Async:        true,
			AsyncTimeout: &timeout,
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		jsonStr := string(data)

		// Verify correct field names in JSON output
		require.Contains(t, jsonStr, `"async":true`)
		require.Contains(t, jsonStr, `"asyncTimeout":5000`)

		require.NotContains(t, jsonStr, `"async_"`)
	})

	t.Run("sync hook output has correct json field names", func(t *testing.T) {
		continueVal := false
		suppressOutput := true
		stopReason := "Testing field conversion"
		decision := "block"
		reason := "Test reason"
		systemMessage := "Test system message"

		output := &SyncHookJSONOutput{
			Continue:       &continueVal,
			SuppressOutput: &suppressOutput,
			StopReason:     &stopReason,
			Decision:       &decision,
			Reason:         &reason,
			SystemMessage:  &systemMessage,
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		jsonStr := string(data)

		// Verify correct field names in JSON output
		require.Contains(t, jsonStr, `"continue":false`)
		require.Contains(t, jsonStr, `"suppressOutput":true`)
		require.Contains(t, jsonStr, `"stopReason":"Testing field conversion"`)
		require.Contains(t, jsonStr, `"decision":"block"`)
		require.Contains(t, jsonStr, `"reason":"Test reason"`)
		require.Contains(t, jsonStr, `"systemMessage":"Test system message"`)

		require.NotContains(t, jsonStr, `"continue_"`)
	})

	t.Run("hook specific output has correct json field names", func(t *testing.T) {
		decision := "deny"
		reason := "Security policy violation"
		output := &SyncHookJSONOutput{
			HookSpecificOutput: &PreToolUseHookSpecificOutput{
				HookEventName:            "PreToolUse",
				PermissionDecision:       &decision,
				PermissionDecisionReason: &reason,
				UpdatedInput:             map[string]any{"modified": "input"},
			},
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		jsonStr := string(data)

		// Verify hook-specific output fields
		require.Contains(t, jsonStr, `"hookEventName":"PreToolUse"`)
		require.Contains(t, jsonStr, `"permissionDecision":"deny"`)
		require.Contains(t, jsonStr, `"permissionDecisionReason":"Security policy violation"`)
		require.Contains(t, jsonStr, `"updatedInput"`)
	})
}
