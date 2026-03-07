package message

import "encoding/json"

const structuredOutputToolName = "StructuredOutput"

// StructuredOutputTracker captures StructuredOutput tool payloads so they can
// be promoted onto the final ResultMessage for compatibility with callers that
// expect structured output there.
type StructuredOutputTracker struct {
	bySession map[string]any
}

// NewStructuredOutputTracker creates a tracker for raw Claude CLI messages.
func NewStructuredOutputTracker() *StructuredOutputTracker {
	return &StructuredOutputTracker{
		bySession: make(map[string]any),
	}
}

// ObserveRaw records StructuredOutput tool invocations from raw assistant
// messages keyed by session ID.
func (t *StructuredOutputTracker) ObserveRaw(raw map[string]any) {
	if t == nil {
		return
	}

	msgType, _ := raw["type"].(string)
	if msgType != messageTypeAssistant {
		return
	}

	sessionID, _ := raw["session_id"].(string)
	if sessionID == "" {
		return
	}

	messageData, _ := raw["message"].(map[string]any)
	contentItems, _ := messageData["content"].([]any)

	for _, item := range contentItems {
		block, _ := item.(map[string]any)
		if block == nil {
			continue
		}

		blockType, _ := block["type"].(string)

		blockName, _ := block["name"].(string)
		if blockType != BlockTypeToolUse || blockName != structuredOutputToolName {
			continue
		}

		if input, ok := block["input"]; ok {
			t.bySession[sessionID] = input
		}
	}
}

// PopulateResult copies a tracked structured output payload onto a result
// message when the CLI emitted it through the StructuredOutput tool instead of
// directly on the result envelope.
func (t *StructuredOutputTracker) PopulateResult(result *ResultMessage) {
	if t == nil || result == nil || result.SessionID == "" {
		return
	}

	output, ok := t.bySession[result.SessionID]
	if !ok {
		return
	}

	delete(t.bySession, result.SessionID)

	if result.StructuredOutput != nil {
		return
	}

	// Only promote cached structured output onto successful terminal results.
	if result.Subtype != "success" || result.IsError {
		return
	}

	result.StructuredOutput = output

	if result.Result == nil {
		if data, err := json.Marshal(output); err == nil {
			value := string(data)
			result.Result = &value
		}
	}
}
