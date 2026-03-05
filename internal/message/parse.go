package message

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/errors"
)

// Parse converts a raw JSON map into a typed Message.
//
// The logger is used to log debug information about message parsing, including
// warnings for unknown message types or malformed data.
//
// Returns an error if the message type is missing, invalid, or if parsing fails.
func Parse(log *slog.Logger, data map[string]any) (Message, error) {
	log = log.With("component", "message_parser")

	msgType, ok := data["type"].(string)
	if !ok {
		log.Debug("Message missing 'type' field")

		return nil, &errors.MessageParseError{
			Message: "missing or invalid 'type' field",
			Err:     fmt.Errorf("missing or invalid 'type' field"),
			Data:    data,
		}
	}

	log.Debug("Parsing message", "message_type", msgType)

	var (
		msg Message
		err error
	)

	switch msgType {
	case "user":
		msg, err = parseUserMessage(data)
	case "assistant":
		msg, err = parseAssistantMessage(data)
	case "system":
		msg, err = parseSystemMessage(data)
	case "result":
		msg, err = parseResultMessage(data)
	case "stream_event":
		msg, err = parseStreamEvent(data)
	default:
		log.Debug("Skipping unknown message type", "message_type", msgType)

		return nil, errors.ErrUnknownMessageType
	}

	if err != nil {
		return nil, &errors.MessageParseError{
			Message: err.Error(),
			Err:     err,
			Data:    data,
		}
	}

	return msg, nil
}

// parseUserMessage parses a UserMessage from raw JSON.
// The wire format has a nested "message" field containing the content.
func parseUserMessage(data map[string]any) (*UserMessage, error) {
	msg := &UserMessage{
		Type: "user",
	}

	// The wire format has a nested "message" field that we flatten
	messageData, ok := data["message"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("user message: missing or invalid 'message' field")
	}

	// Parse content field using UserMessageContent which handles both string and array
	contentData, ok := messageData["content"]
	if !ok {
		return nil, fmt.Errorf("user message: missing content field")
	}

	// Marshal content back to JSON for UserMessageContent.UnmarshalJSON
	contentJSON, err := json.Marshal(contentData)
	if err != nil {
		return nil, fmt.Errorf("user message: marshal content: %w", err)
	}

	var content UserMessageContent
	if err := json.Unmarshal(contentJSON, &content); err != nil {
		return nil, fmt.Errorf("user message: %w", err)
	}

	msg.Content = content

	// uuid and parent_tool_use_id stay at top level (outer data)
	if uuid, ok := data["uuid"].(string); ok {
		msg.UUID = &uuid
	}

	if parentToolUseID, ok := data["parent_tool_use_id"].(string); ok {
		msg.ParentToolUseID = &parentToolUseID
	}

	return msg, nil
}

// parseAssistantMessage parses an AssistantMessage from raw JSON.
func parseAssistantMessage(data map[string]any) (*AssistantMessage, error) {
	msg := &AssistantMessage{
		Type: "assistant",
	}

	// The wire format has a nested "message" field that we flatten
	messageData, ok := data["message"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'message' field")
	}

	// Parse content blocks
	if contentData, ok := messageData["content"].([]any); ok {
		content, err := parseContentBlocks(contentData)
		if err != nil {
			return nil, fmt.Errorf("parse assistant content: %w", err)
		}

		msg.Content = content
	}

	// Parse model
	if model, ok := messageData["model"].(string); ok {
		msg.Model = model
	}

	// Parse parent_tool_use_id from outer data (not messageData)
	if parentToolUseID, ok := data["parent_tool_use_id"].(string); ok {
		msg.ParentToolUseID = &parentToolUseID
	}

	// Parse error from outer data (not messageData) — CLI puts error at top level
	if errorVal, ok := data["error"].(string); ok {
		errType := AssistantMessageError(errorVal)
		msg.Error = &errType
	}

	return msg, nil
}

// parseSystemMessage parses a SystemMessage from raw JSON.
func parseSystemMessage(data map[string]any) (*SystemMessage, error) {
	msg := &SystemMessage{
		Type: "system",
	}

	// Validate required subtype field
	subtype, ok := data["subtype"].(string)
	if !ok {
		return nil, fmt.Errorf("system message: missing or invalid 'subtype' field")
	}

	msg.Subtype = subtype

	// For init messages, capture all fields (agents, tools, etc.) into Data
	// The CLI sends these at the root level, not in a nested "data" field
	if msgData, ok := data["data"].(map[string]any); ok {
		msg.Data = msgData
	} else {
		// Capture all non-standard fields into Data
		msg.Data = make(map[string]any)

		for k, v := range data {
			if k != "type" && k != "subtype" {
				msg.Data[k] = v
			}
		}
	}

	return msg, nil
}

// parseStreamEvent parses a StreamEvent from raw JSON.
func parseStreamEvent(data map[string]any) (*StreamEvent, error) {
	event := &StreamEvent{}

	uuid, ok := data["uuid"].(string)
	if !ok {
		return nil, fmt.Errorf("stream_event: missing or invalid 'uuid' field")
	}

	event.UUID = uuid

	sessionID, ok := data["session_id"].(string)
	if !ok {
		return nil, fmt.Errorf("stream_event: missing or invalid 'session_id' field")
	}

	event.SessionID = sessionID

	eventData, ok := data["event"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("stream_event: missing or invalid 'event' field")
	}

	event.Event = eventData

	// Optional field
	if parentToolUseID, ok := data["parent_tool_use_id"].(string); ok {
		event.ParentToolUseID = &parentToolUseID
	}

	return event, nil
}

// parseResultMessage parses a ResultMessage from raw JSON.
func parseResultMessage(data map[string]any) (*ResultMessage, error) {
	// Validate required subtype field
	if _, ok := data["subtype"].(string); !ok {
		return nil, fmt.Errorf("result message: missing or invalid 'subtype' field")
	}

	// Re-marshal and unmarshal to use json struct tags for proper parsing
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	var msg ResultMessage
	if err := json.Unmarshal(jsonBytes, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &msg, nil
}

// parseContentBlocks parses an array of content blocks.
func parseContentBlocks(data []any) ([]ContentBlock, error) {
	blocks := make([]ContentBlock, 0, len(data))

	for i, item := range data {
		blockData, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("content block %d: not an object", i)
		}

		block, err := parseContentBlock(blockData)
		if err != nil {
			return nil, fmt.Errorf("content block %d: %w", i, err)
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}

// parseContentBlock parses a single content block.
func parseContentBlock(data map[string]any) (ContentBlock, error) {
	blockType, ok := data["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'type' field")
	}

	switch blockType {
	case "text":
		return parseTextBlock(data)
	case "thinking":
		return parseThinkingBlock(data)
	case "tool_use":
		return parseToolUseBlock(data)
	case "tool_result":
		return parseToolResultBlock(data)
	default:
		// Fall back to a generic TextBlock for unknown types (forward-compatible
		// with new CLI content block types), matching UnmarshalContentBlock behavior.
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal unknown content block: %w", err)
		}

		var block TextBlock
		if err := json.Unmarshal(jsonBytes, &block); err != nil {
			return nil, fmt.Errorf("unmarshal unknown content block as text: %w", err)
		}

		return &block, nil
	}
}

// parseTextBlock parses a TextBlock from raw JSON.
func parseTextBlock(data map[string]any) (*TextBlock, error) {
	block := &TextBlock{
		Type: "text",
	}

	if text, ok := data["text"].(string); ok {
		block.Text = text
	}

	return block, nil
}

// parseThinkingBlock parses a ThinkingBlock from raw JSON.
func parseThinkingBlock(data map[string]any) (*ThinkingBlock, error) {
	block := &ThinkingBlock{
		Type: "thinking",
	}

	if thinking, ok := data["thinking"].(string); ok {
		block.Thinking = thinking
	}

	if signature, ok := data["signature"].(string); ok {
		block.Signature = signature
	}

	return block, nil
}

// parseToolUseBlock parses a ToolUseBlock from raw JSON.
func parseToolUseBlock(data map[string]any) (*ToolUseBlock, error) {
	block := &ToolUseBlock{
		Type: "tool_use",
	}

	if id, ok := data["id"].(string); ok {
		block.ID = id
	}

	if name, ok := data["name"].(string); ok {
		block.Name = name
	}

	if input, ok := data["input"].(map[string]any); ok {
		block.Input = input
	}

	return block, nil
}

// parseToolResultBlock parses a ToolResultBlock from raw JSON.
func parseToolResultBlock(data map[string]any) (*ToolResultBlock, error) {
	block := &ToolResultBlock{
		Type: "tool_result",
	}

	if toolUseID, ok := data["tool_use_id"].(string); ok {
		block.ToolUseID = toolUseID
	}

	if isError, ok := data["is_error"].(bool); ok {
		block.IsError = isError
	}

	// Parse content if present
	if contentData, ok := data["content"].([]any); ok {
		content, err := parseContentBlocks(contentData)
		if err != nil {
			return nil, fmt.Errorf("parse tool result content: %w", err)
		}

		block.Content = content
	}

	return block, nil
}
