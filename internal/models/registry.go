package models

// allCapabilities is the set of capabilities shared by all current Claude models.
var allCapabilities = []Capability{
	CapVision,
	CapToolUse,
	CapReasoning,
	CapStructuredOutput,
}

// registry is the internal list of all known Claude models.
var registry = []Model{
	{
		ID:              modelIDClaudeOpus46,
		Name:            "Claude Opus 4.6",
		CostTier:        CostTierHigh,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 128_000,
	},
	{
		ID:              modelIDClaudeSonnet46,
		Name:            "Claude Sonnet 4.6",
		CostTier:        CostTierMedium,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 64_000,
	},
	{
		ID:              modelIDClaudeHaiku45,
		Name:            "Claude Haiku 4.5",
		CostTier:        CostTierLow,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 64_000,
	},
}
