package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructuredOutputTrackerPopulatesResultMessage(t *testing.T) {
	tracker := NewStructuredOutputTracker()

	tracker.ObserveRaw(map[string]any{
		"type":       "assistant",
		"session_id": "session-1",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "StructuredOutput",
					"input": map[string]any{"answer": "4", "confidence": 0.99},
				},
			},
		},
	})

	result := &ResultMessage{
		Type:      "result",
		Subtype:   "success",
		SessionID: "session-1",
	}

	tracker.PopulateResult(result)

	require.Equal(t, map[string]any{"answer": "4", "confidence": 0.99}, result.StructuredOutput)
	require.NotNil(t, result.Result)
	require.JSONEq(t, `{"answer":"4","confidence":0.99}`, *result.Result)
}

func TestStructuredOutputTrackerIgnoresUnrelatedMessages(t *testing.T) {
	tracker := NewStructuredOutputTracker()

	tracker.ObserveRaw(map[string]any{
		"type":       "assistant",
		"session_id": "session-1",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "Write",
					"input": map[string]any{"file_path": "/tmp/test.txt"},
				},
			},
		},
	})

	result := &ResultMessage{
		Type:      "result",
		Subtype:   "success",
		SessionID: "session-1",
	}

	tracker.PopulateResult(result)

	require.Nil(t, result.StructuredOutput)
	require.Nil(t, result.Result)
}

func TestStructuredOutputTrackerDoesNotOverrideExistingStructuredOutput(t *testing.T) {
	tracker := NewStructuredOutputTracker()

	tracker.ObserveRaw(map[string]any{
		"type":       "assistant",
		"session_id": "session-1",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "StructuredOutput",
					"input": map[string]any{"answer": "4"},
				},
			},
		},
	})

	existingResult := `{"answer":"already-present"}`
	result := &ResultMessage{
		Type:             "result",
		Subtype:          "success",
		SessionID:        "session-1",
		Result:           &existingResult,
		StructuredOutput: map[string]any{"answer": "already-present"},
	}

	tracker.PopulateResult(result)

	require.Equal(t, map[string]any{"answer": "already-present"}, result.StructuredOutput)
	require.Equal(t, existingResult, *result.Result)
}

func TestStructuredOutputTrackerClearsStateWhenResultAlreadyHasStructuredOutput(t *testing.T) {
	tracker := NewStructuredOutputTracker()

	tracker.ObserveRaw(map[string]any{
		"type":       "assistant",
		"session_id": "session-1",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "StructuredOutput",
					"input": map[string]any{"answer": "stale"},
				},
			},
		},
	})

	existingResult := `{"answer":"fresh"}`
	result := &ResultMessage{
		Type:             "result",
		Subtype:          "success",
		SessionID:        "session-1",
		Result:           &existingResult,
		StructuredOutput: map[string]any{"answer": "fresh"},
	}

	tracker.PopulateResult(result)

	_, exists := tracker.bySession["session-1"]
	require.False(t, exists, "tracker should discard cached output after the session result is seen")

	nextResult := &ResultMessage{
		Type:      "result",
		Subtype:   "success",
		SessionID: "session-1",
	}

	tracker.PopulateResult(nextResult)

	require.Nil(t, nextResult.StructuredOutput, "stale structured output must not leak into later results")
	require.Nil(t, nextResult.Result)
}

func TestStructuredOutputTrackerIgnoresMessagesWithoutSessionID(t *testing.T) {
	tracker := NewStructuredOutputTracker()

	tracker.ObserveRaw(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "StructuredOutput",
					"input": map[string]any{"answer": "4"},
				},
			},
		},
	})

	result := &ResultMessage{
		Type:      "result",
		Subtype:   "success",
		SessionID: "session-1",
	}

	tracker.PopulateResult(result)

	require.Nil(t, result.StructuredOutput)
	require.Nil(t, result.Result)
}

func TestStructuredOutputTrackerDoesNotPopulateNonSuccessResult(t *testing.T) {
	tracker := NewStructuredOutputTracker()

	tracker.ObserveRaw(map[string]any{
		"type":       "assistant",
		"session_id": "session-1",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "StructuredOutput",
					"input": map[string]any{"answer": "stale"},
				},
			},
		},
	})

	result := &ResultMessage{
		Type:      "result",
		Subtype:   "error_max_budget_usd",
		SessionID: "session-1",
	}

	tracker.PopulateResult(result)

	require.Nil(t, result.StructuredOutput, "structured output should not be backfilled onto non-success terminal results")
	require.Nil(t, result.Result, "result text should not be synthesized for non-success terminal results")

	_, exists := tracker.bySession["session-1"]
	require.False(t, exists, "tracker should still discard cached output after any terminal result")
}
