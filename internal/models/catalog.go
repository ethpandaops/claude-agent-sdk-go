// Package models provides a catalog of known Claude models and their
// capabilities. It is the source of truth for model metadata within the SDK.
package models

import (
	"slices"
)

// Capability represents a model capability such as vision or tool use.
type Capability string

const (
	// CapVision indicates the model supports image/vision inputs.
	CapVision Capability = "vision"
	// CapToolUse indicates the model supports tool/function calling.
	CapToolUse Capability = "tool-use"
	// CapReasoning indicates the model supports extended reasoning.
	CapReasoning Capability = "reasoning"
	// CapStructuredOutput indicates the model supports structured JSON output.
	CapStructuredOutput Capability = "structured-output"
)

// CostTier represents a provider-agnostic relative cost tier.
type CostTier string

const (
	// CostTierHigh represents opus-class pricing.
	CostTierHigh CostTier = "high"
	// CostTierMedium represents sonnet-class pricing.
	CostTierMedium CostTier = "medium"
	// CostTierLow represents haiku-class pricing.
	CostTierLow CostTier = "low"
)

// Model holds metadata for a single Claude model.
type Model struct {
	// ID is the API model identifier (e.g. "claude-opus-4-6").
	ID string
	// Name is the human-readable display name.
	Name string
	// CostTier is the relative cost tier for this model.
	CostTier CostTier
	// Capabilities lists what the model supports.
	Capabilities []Capability
	// ContextWindow is the default context window size in tokens.
	ContextWindow int
	// MaxOutputTokens is the maximum number of output tokens.
	MaxOutputTokens int
}

// HasCapability reports whether the model supports the given capability.
func (m Model) HasCapability(capability Capability) bool {
	return slices.Contains(m.Capabilities, capability)
}

// CapabilityStrings returns capabilities as a string slice for interop
// with string-based systems.
func (m Model) CapabilityStrings() []string {
	out := make([]string, 0, len(m.Capabilities))
	for _, c := range m.Capabilities {
		out = append(out, string(c))
	}

	return out
}

// All returns a copy of every known model in the static catalog.
// This is not a live account-specific list from the Claude CLI.
func All() []Model {
	out := make([]Model, len(registry))
	copy(out, registry)

	return out
}

// ByID looks up a model by its identifier.
// Returns nil if no model is found.
func ByID(id string) *Model {
	// Exact ID match.
	for i := range registry {
		if registry[i].ID == id {
			m := registry[i]

			return &m
		}
	}

	return nil
}

// ByCostTier returns all models matching the given cost tier.
func ByCostTier(tier CostTier) []Model {
	var out []Model

	for _, m := range registry {
		if m.CostTier == tier {
			out = append(out, m)
		}
	}

	return out
}

// Capabilities is a convenience function that returns capability strings
// for the given model ID. Returns nil if the model is not found.
func Capabilities(modelID string) []string {
	m := ByID(modelID)
	if m == nil {
		return nil
	}

	return m.CapabilityStrings()
}
