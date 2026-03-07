// Package message provides message and content block types for Claude conversations.
package message

import (
	"encoding/json"
	"fmt"
)

// Block type constants.
const (
	BlockTypeText       = "text"
	BlockTypeThinking   = "thinking"
	BlockTypeToolUse    = "tool_use"
	BlockTypeToolResult = "tool_result"
	BlockTypeToolRef    = "tool_reference"
)

// ContentBlock represents a block of content within a message.
type ContentBlock interface {
	BlockType() string
}

// Compile-time verification that all content block types implement ContentBlock.
var (
	_ ContentBlock = (*TextBlock)(nil)
	_ ContentBlock = (*ThinkingBlock)(nil)
	_ ContentBlock = (*ToolUseBlock)(nil)
	_ ContentBlock = (*ToolResultBlock)(nil)
	_ ContentBlock = (*ToolReferenceBlock)(nil)
	_ ContentBlock = (*UnknownBlock)(nil)
)

// TextBlock contains plain text content.
type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// BlockType implements the ContentBlock interface.
func (b *TextBlock) BlockType() string { return BlockTypeText }

// ThinkingBlock contains Claude's thinking process.
type ThinkingBlock struct {
	Type      string `json:"type"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// BlockType implements the ContentBlock interface.
func (b *ThinkingBlock) BlockType() string { return BlockTypeThinking }

// ToolUseBlock represents Claude using a tool.
type ToolUseBlock struct {
	Type  string         `json:"type"`
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// BlockType implements the ContentBlock interface.
func (b *ToolUseBlock) BlockType() string { return BlockTypeToolUse }

// ToolResultBlock contains the result of a tool execution.
//
//nolint:tagliatelle // Claude CLI uses snake_case for JSON fields
type ToolResultBlock struct {
	Type      string         `json:"type"`
	ToolUseID string         `json:"tool_use_id"`
	Content   []ContentBlock `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

// BlockType implements the ContentBlock interface.
func (b *ToolResultBlock) BlockType() string { return BlockTypeToolResult }

// ToolReferenceBlock points to a deferred tool selected by Claude tool search.
//
//nolint:tagliatelle // Claude CLI uses snake_case for JSON fields
type ToolReferenceBlock struct {
	Type     string `json:"type"`
	ToolName string `json:"tool_name"`
}

// BlockType implements the ContentBlock interface.
func (b *ToolReferenceBlock) BlockType() string { return BlockTypeToolRef }

// UnknownBlock preserves unrecognized content block payloads without failing parsing.
type UnknownBlock struct {
	Type string         `json:"type"`
	Raw  map[string]any `json:"raw,omitempty"`
}

// BlockType implements the ContentBlock interface.
func (b *UnknownBlock) BlockType() string { return b.Type }

// MarshalJSON preserves the original block payload for round-tripping.
func (b *UnknownBlock) MarshalJSON() ([]byte, error) {
	if b == nil {
		return json.Marshal(nil)
	}

	if len(b.Raw) > 0 {
		raw := make(map[string]any, len(b.Raw)+1)
		for key, value := range b.Raw {
			raw[key] = value
		}

		if _, ok := raw["type"]; !ok && b.Type != "" {
			raw["type"] = b.Type
		}

		return json.Marshal(raw)
	}

	type wireUnknownBlock struct {
		Type string `json:"type"`
	}

	return json.Marshal(wireUnknownBlock{Type: b.Type})
}

// UnmarshalJSON implements json.Unmarshaler for ToolResultBlock.
// Claude CLI tool results may arrive as a plain string or structured blocks.
func (b *ToolResultBlock) UnmarshalJSON(data []byte) error {
	type Alias ToolResultBlock

	aux := &struct {
		Content json.RawMessage `json:"content,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(b),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if len(aux.Content) == 0 || string(aux.Content) == "null" {
		return nil
	}

	// Try string first
	var text string
	if err := json.Unmarshal(aux.Content, &text); err == nil {
		b.Content = []ContentBlock{&TextBlock{Type: BlockTypeText, Text: text}}

		return nil
	}

	// Try array of blocks
	var rawBlocks []json.RawMessage
	if err := json.Unmarshal(aux.Content, &rawBlocks); err != nil {
		return err
	}

	b.Content = make([]ContentBlock, 0, len(rawBlocks))

	for _, raw := range rawBlocks {
		block, err := UnmarshalContentBlock(raw)
		if err != nil {
			return err
		}

		b.Content = append(b.Content, block)
	}

	return nil
}

// UnmarshalContentBlock unmarshals a single content block from JSON.
func UnmarshalContentBlock(data []byte) (ContentBlock, error) {
	var typeHolder struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &typeHolder); err != nil {
		return nil, err
	}

	if typeHolder.Type == "" {
		return nil, fmt.Errorf("missing or invalid 'type' field")
	}

	switch typeHolder.Type {
	case BlockTypeText:
		var block TextBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}

		return &block, nil
	case BlockTypeThinking:
		var block ThinkingBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}

		return &block, nil
	case BlockTypeToolUse:
		var block ToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}

		return &block, nil
	case BlockTypeToolResult:
		var block ToolResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}

		return &block, nil
	case BlockTypeToolRef:
		var block ToolReferenceBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}

		return &block, nil
	default:
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("unknown content block type %q: %w", typeHolder.Type, err)
		}

		return &UnknownBlock{
			Type: typeHolder.Type,
			Raw:  raw,
		}, nil
	}
}
