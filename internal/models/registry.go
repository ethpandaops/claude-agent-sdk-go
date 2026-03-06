package models

// allCapabilities is the set of capabilities shared by all current Claude models.
var allCapabilities = []Capability{
	CapVision,
	CapToolUse,
	CapReasoning,
	CapStructuredOutput,
}

// registry is the internal list of all known Claude models.
// Only the latest model per tier gets the short alias.
var registry = []Model{
	{
		ID:              modelIDClaudeOpus46,
		Name:            "Claude Opus 4.6",
		Aliases:         []string{"opus"},
		CostTier:        CostTierHigh,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 128_000,
	},
	{
		ID:              modelIDClaudeSonnet46,
		Name:            "Claude Sonnet 4.6",
		Aliases:         []string{"sonnet"},
		CostTier:        CostTierMedium,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 64_000,
	},
	{
		ID:              modelIDSonnet1M,
		Name:            "Claude Sonnet 4.6 (1M context alias)",
		CostTier:        CostTierMedium,
		Capabilities:    allCapabilities,
		ContextWindow:   1_000_000,
		MaxOutputTokens: 64_000,
	},
	{
		ID:              modelIDOpus1M,
		Name:            "Claude Opus 4.6 (1M context alias)",
		CostTier:        CostTierHigh,
		Capabilities:    allCapabilities,
		ContextWindow:   1_000_000,
		MaxOutputTokens: 128_000,
	},
	{
		ID:              "claude-haiku-4-5",
		Name:            "Claude Haiku 4.5",
		Aliases:         []string{"haiku"},
		CostTier:        CostTierLow,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 64_000,
	},
	{
		ID:              "claude-opus-4-5",
		Name:            "Claude Opus 4.5",
		CostTier:        CostTierHigh,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 64_000,
	},
	{
		ID:              "claude-sonnet-4-5",
		Name:            "Claude Sonnet 4.5",
		CostTier:        CostTierMedium,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 64_000,
	},
	{
		ID:              "claude-opus-4-1",
		Name:            "Claude Opus 4.1",
		CostTier:        CostTierHigh,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 32_000,
	},
	{
		ID:              "claude-opus-4-0",
		Name:            "Claude Opus 4",
		CostTier:        CostTierHigh,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 32_000,
	},
	{
		ID:              "claude-sonnet-4-0",
		Name:            "Claude Sonnet 4",
		CostTier:        CostTierMedium,
		Capabilities:    allCapabilities,
		ContextWindow:   200_000,
		MaxOutputTokens: 64_000,
	},
}
