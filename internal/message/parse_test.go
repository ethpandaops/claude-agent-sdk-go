package message

import (
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	sdkerrors "github.com/ethpandaops/claude-agent-sdk-go/internal/errors"

	"github.com/stretchr/testify/require"
)

func TestParseAssistantMessage(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name           string
		data           map[string]any
		wantError      bool
		wantParseErr   bool
		wantErrorValue AssistantMessageError
		wantModel      string
		wantContentLen int
		wantToolUseID  *string
	}{
		{
			name: "no error field",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{
						map[string]any{"type": "text", "text": "hello"},
					},
					"model": "claude-sonnet-4-6",
				},
			},
			wantError:      false,
			wantModel:      "claude-sonnet-4-6",
			wantContentLen: 1,
		},
		{
			name: "authentication_failed error",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{},
					"model":   "claude-sonnet-4-6",
				},
				"error": "authentication_failed",
			},
			wantError:      true,
			wantErrorValue: AssistantMessageErrorAuthFailed,
			wantModel:      "claude-sonnet-4-6",
			wantContentLen: 0,
		},
		{
			name: "rate_limit error",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{},
					"model":   "claude-sonnet-4-6",
				},
				"error": "rate_limit",
			},
			wantError:      true,
			wantErrorValue: AssistantMessageErrorRateLimit,
			wantModel:      "claude-sonnet-4-6",
			wantContentLen: 0,
		},
		{
			name: "unknown error",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{},
					"model":   "claude-sonnet-4-6",
				},
				"error": "unknown",
			},
			wantError:      true,
			wantErrorValue: AssistantMessageErrorUnknown,
			wantModel:      "claude-sonnet-4-6",
			wantContentLen: 0,
		},
		{
			name: "error at top level not in nested message",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{
						map[string]any{"type": "text", "text": "partial response"},
					},
					"model": "claude-sonnet-4-6",
					"error": "should_be_ignored",
				},
				"error":              "billing_error",
				"parent_tool_use_id": "tool-123",
			},
			wantError:      true,
			wantErrorValue: AssistantMessageErrorBilling,
			wantModel:      "claude-sonnet-4-6",
			wantContentLen: 1,
			wantToolUseID:  new("tool-123"),
		},
		{
			name: "missing message field returns parse error",
			data: map[string]any{
				"type": "assistant",
			},
			wantParseErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := Parse(logger, tt.data)

			if tt.wantParseErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			assistant, ok := msg.(*AssistantMessage)
			require.True(t, ok, "expected *AssistantMessage")
			require.Equal(t, "assistant", assistant.Type)
			require.Equal(t, tt.wantModel, assistant.Model)
			require.Len(t, assistant.Content, tt.wantContentLen)

			if tt.wantError {
				require.NotNil(t, assistant.Error)
				require.Equal(t, tt.wantErrorValue, *assistant.Error)
			} else {
				require.Nil(t, assistant.Error)
			}

			if tt.wantToolUseID != nil {
				require.NotNil(t, assistant.ParentToolUseID)
				require.Equal(t, *tt.wantToolUseID, *assistant.ParentToolUseID)
			}
		})
	}
}

func TestParseUnknownMessageTypes(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name    string
		data    map[string]any
		wantErr error
	}{
		{
			name: "rate_limit_event with warning",
			data: map[string]any{
				"type":   "rate_limit_event",
				"status": "allowed_warning",
				"message": "You are approaching your rate limit. " +
					"Please slow down.",
			},
			wantErr: sdkerrors.ErrUnknownMessageType,
		},
		{
			name: "rate_limit_event with rejected status",
			data: map[string]any{
				"type":    "rate_limit_event",
				"status":  "rejected",
				"message": "Rate limit exceeded. Please wait.",
			},
			wantErr: sdkerrors.ErrUnknownMessageType,
		},
		{
			name: "arbitrary unknown type",
			data: map[string]any{
				"type": "some_future_event_type",
				"data": map[string]any{"key": "value"},
			},
			wantErr: sdkerrors.ErrUnknownMessageType,
		},
		{
			name:    "missing type field returns MessageParseError",
			data:    map[string]any{"data": "no type here"},
			wantErr: nil, // checked separately below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := Parse(logger, tt.data)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, msg)

				return
			}

			// "missing type field" case: expect MessageParseError
			require.Error(t, err)
			require.Nil(t, msg)

			_, ok := errors.AsType[*sdkerrors.MessageParseError](err)
			require.True(t, ok,
				"expected *MessageParseError, got %T", err)
		})
	}
}

func TestParseUnknownContentBlockType(t *testing.T) {
	logger := slog.Default()

	data := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type": "some_new_block_type",
					"text": "fallback text content",
				},
				map[string]any{
					"type": "text",
					"text": "normal text",
				},
			},
			"model": "claude-sonnet-4-6",
		},
	}

	msg, err := Parse(logger, data)
	require.NoError(t, err)

	assistant, ok := msg.(*AssistantMessage)
	require.True(t, ok)
	require.Len(t, assistant.Content, 2)

	unknown, ok := assistant.Content[0].(*UnknownBlock)
	require.True(t, ok)
	require.Equal(t, "some_new_block_type", unknown.Type)
	require.Equal(t, "fallback text content", unknown.Raw["text"])
}

func TestParseAssistantMessage_LiveThinkingBlock(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	msg, err := Parse(logger, map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":      "thinking",
					"thinking":  "The user wants structured output for a simple math question.",
					"signature": "sig_live_cli",
				},
			},
			"model": "claude-sonnet-4-6",
		},
	})
	require.NoError(t, err)

	assistant, ok := msg.(*AssistantMessage)
	require.True(t, ok)
	require.Len(t, assistant.Content, 1)

	thinking, ok := assistant.Content[0].(*ThinkingBlock)
	require.True(t, ok)
	require.Equal(t, "The user wants structured output for a simple math question.", thinking.Thinking)
	require.Equal(t, "sig_live_cli", thinking.Signature)
}

func TestParseUserMessage_ToolReferenceBlock(t *testing.T) {
	t.Parallel()

	msg, err := parseUserMessage(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "toolu_123",
					"content": []any{
						map[string]any{
							"type":      "tool_reference",
							"tool_name": "TaskOutput",
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	blocks := msg.Content.Blocks()
	require.Len(t, blocks, 1)

	tr, ok := blocks[0].(*ToolResultBlock)
	require.True(t, ok)
	require.Len(t, tr.Content, 1)

	ref, ok := tr.Content[0].(*ToolReferenceBlock)
	require.True(t, ok)
	require.Equal(t, "TaskOutput", ref.ToolName)
}

func TestParseUserMessage_LiveAgentToolResult(t *testing.T) {
	t.Parallel()

	msg, err := parseUserMessage(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "toolu_agent",
					"content": []any{
						map[string]any{
							"type": "text",
							"text": "/tmp/tmp.ATcDR5f0H5",
						},
						map[string]any{
							"type": "text",
							"text": "agentId: adde1ae7843ebe607",
						},
					},
				},
			},
		},
		"tool_use_result": map[string]any{
			"status":  "completed",
			"agentId": "adde1ae7843ebe607",
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "/tmp/tmp.ATcDR5f0H5",
				},
			},
		},
	})
	require.NoError(t, err)

	blocks := msg.Content.Blocks()
	require.Len(t, blocks, 1)

	tr, ok := blocks[0].(*ToolResultBlock)
	require.True(t, ok)
	require.Len(t, tr.Content, 2)

	firstText, ok := tr.Content[0].(*TextBlock)
	require.True(t, ok)
	require.Equal(t, "/tmp/tmp.ATcDR5f0H5", firstText.Text)

	require.Equal(t, "completed", msg.ToolUseResult["status"])
	require.Equal(t, "adde1ae7843ebe607", msg.ToolUseResult["agentId"])
}

func TestUserMessageContent_UnmarshalString(t *testing.T) {
	var content UserMessageContent

	err := json.Unmarshal([]byte(`"ping"`), &content)
	require.NoError(t, err)
	require.True(t, content.IsString())
	require.Equal(t, "ping", content.String())
	require.Len(t, content.Blocks(), 1)
}

func TestToolResultBlock_UnmarshalStringContent(t *testing.T) {
	var block ToolResultBlock

	err := json.Unmarshal([]byte(`{
		"type": "tool_result",
		"tool_use_id": "toolu_123",
		"content": "Structured output provided successfully"
	}`), &block)
	require.NoError(t, err)
	require.Equal(t, "toolu_123", block.ToolUseID)
	require.Len(t, block.Content, 1)

	textBlock, ok := block.Content[0].(*TextBlock)
	require.True(t, ok)
	require.Equal(t, "Structured output provided successfully", textBlock.Text)
}

func TestParseTaskSystemMessages(t *testing.T) {
	logger := slog.Default()

	taskStarted, err := Parse(logger, map[string]any{
		"type":        "system",
		"subtype":     "task_started",
		"task_id":     "task-1",
		"description": "Run a task",
		"uuid":        "msg-1",
		"session_id":  "session-1",
		"task_type":   "research",
	})
	require.NoError(t, err)

	started, ok := taskStarted.(*TaskStartedMessage)
	require.True(t, ok)
	require.Equal(t, "task-1", started.TaskID)
	require.NotNil(t, started.TaskType)
	require.Equal(t, "research", *started.TaskType)

	taskProgress, err := Parse(logger, map[string]any{
		"type":           "system",
		"subtype":        "task_progress",
		"task_id":        "task-1",
		"description":    "Still running",
		"uuid":           "msg-2",
		"session_id":     "session-1",
		"last_tool_name": "Read",
		"usage": map[string]any{
			"total_tokens": 12,
			"tool_uses":    3,
			"duration_ms":  44,
		},
	})
	require.NoError(t, err)

	progress, ok := taskProgress.(*TaskProgressMessage)
	require.True(t, ok)
	require.Equal(t, 12, progress.Usage.TotalTokens)
	require.NotNil(t, progress.LastToolName)
	require.Equal(t, "Read", *progress.LastToolName)

	taskNotification, err := Parse(logger, map[string]any{
		"type":        "system",
		"subtype":     "task_notification",
		"task_id":     "task-1",
		"status":      "stopped",
		"output_file": "/tmp/task.txt",
		"summary":     "Stopped by user",
		"uuid":        "msg-3",
		"session_id":  "session-1",
	})
	require.NoError(t, err)

	notification, ok := taskNotification.(*TaskNotificationMessage)
	require.True(t, ok)
	require.Equal(t, TaskNotificationStatusStopped, notification.Status)
	require.Nil(t, notification.Usage)
}

func TestParseResultMessageStopReason(t *testing.T) {
	logger := slog.Default()

	msg, err := Parse(logger, map[string]any{
		"type":            "result",
		"subtype":         "success",
		"duration_ms":     10,
		"duration_api_ms": 5,
		"is_error":        false,
		"num_turns":       1,
		"session_id":      "session-1",
		"stop_reason":     "end_turn",
	})
	require.NoError(t, err)

	result, ok := msg.(*ResultMessage)
	require.True(t, ok)
	require.NotNil(t, result.StopReason)
	require.Equal(t, "end_turn", *result.StopReason)
}

func TestParseResultMessageStructuredOutput(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	msg, err := Parse(logger, map[string]any{
		"type":            "result",
		"subtype":         "success",
		"duration_ms":     10,
		"duration_api_ms": 5,
		"is_error":        false,
		"num_turns":       1,
		"session_id":      "session-1",
		"result":          "2 + 2 = **4**",
		"structured_output": map[string]any{
			"answer":     "4",
			"confidence": 1.0,
		},
	})
	require.NoError(t, err)

	result, ok := msg.(*ResultMessage)
	require.True(t, ok)
	require.Equal(t, "2 + 2 = **4**", *result.Result)
	require.Equal(t, map[string]any{"answer": "4", "confidence": 1.0}, result.StructuredOutput)
}

func TestParseSystemInitMessage(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	msg, err := Parse(logger, map[string]any{
		"type":       "system",
		"subtype":    "init",
		"session_id": "session-1",
		"model":      "claude-sonnet-4-6",
		"tools":      []any{"Read", "Write", "StructuredOutput"},
	})
	require.NoError(t, err)

	system, ok := msg.(*SystemMessage)
	require.True(t, ok)
	require.Equal(t, "init", system.Subtype)
	require.Equal(t, "session-1", system.Data["session_id"])
}

func TestParseStreamEvent_LiveDeltas(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	msg, err := Parse(logger, map[string]any{
		"type":       "stream_event",
		"uuid":       "evt-1",
		"session_id": "session-1",
		"event": map[string]any{
			"type":  "content_block_delta",
			"index": 1,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": "{\"answer\":",
			},
		},
	})
	require.NoError(t, err)

	event, ok := msg.(*StreamEvent)
	require.True(t, ok)
	require.Equal(t, "content_block_delta", event.Event["type"])

	delta, ok := event.Event["delta"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "input_json_delta", delta["type"])
	require.Equal(t, "{\"answer\":", delta["partial_json"])
}
