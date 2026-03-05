package message

import (
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
					"model": "claude-sonnet-4-5-20250514",
				},
			},
			wantError:      false,
			wantModel:      "claude-sonnet-4-5-20250514",
			wantContentLen: 1,
		},
		{
			name: "authentication_failed error",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{},
					"model":   "claude-sonnet-4-5-20250514",
				},
				"error": "authentication_failed",
			},
			wantError:      true,
			wantErrorValue: AssistantMessageErrorAuthFailed,
			wantModel:      "claude-sonnet-4-5-20250514",
			wantContentLen: 0,
		},
		{
			name: "rate_limit error",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{},
					"model":   "claude-sonnet-4-5-20250514",
				},
				"error": "rate_limit",
			},
			wantError:      true,
			wantErrorValue: AssistantMessageErrorRateLimit,
			wantModel:      "claude-sonnet-4-5-20250514",
			wantContentLen: 0,
		},
		{
			name: "unknown error",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"content": []any{},
					"model":   "claude-sonnet-4-5-20250514",
				},
				"error": "unknown",
			},
			wantError:      true,
			wantErrorValue: AssistantMessageErrorUnknown,
			wantModel:      "claude-sonnet-4-5-20250514",
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
					"model": "claude-sonnet-4-5-20250514",
					"error": "should_be_ignored",
				},
				"error":              "billing_error",
				"parent_tool_use_id": "tool-123",
			},
			wantError:      true,
			wantErrorValue: AssistantMessageErrorBilling,
			wantModel:      "claude-sonnet-4-5-20250514",
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

	// An assistant message containing an unknown content block type
	// should parse successfully with the unknown block falling back to TextBlock.
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
			"model": "claude-sonnet-4-5-20250514",
		},
	}

	msg, err := Parse(logger, data)
	require.NoError(t, err)

	assistant, ok := msg.(*AssistantMessage)
	require.True(t, ok, "expected *AssistantMessage")
	require.Len(t, assistant.Content, 2)

	// Unknown block type falls back to TextBlock
	fallback, ok := assistant.Content[0].(*TextBlock)
	require.True(t, ok, "expected unknown block to fall back to *TextBlock")
	require.Equal(t, "fallback text content", fallback.Text)

	// Normal text block still works
	textBlock, ok := assistant.Content[1].(*TextBlock)
	require.True(t, ok, "expected *TextBlock")
	require.Equal(t, "normal text", textBlock.Text)
}
