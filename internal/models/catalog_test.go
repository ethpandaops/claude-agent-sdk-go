package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAll(t *testing.T) {
	all := All()
	require.NotEmpty(t, all, "catalog must not be empty")

	for _, m := range all {
		assert.NotEmpty(t, m.ID, "model ID must not be empty")
		assert.NotEmpty(t, m.Name, "model Name must not be empty")
		assert.NotEmpty(t, m.CostTier, "model CostTier must not be empty")
		assert.NotEmpty(t, m.Capabilities, "model Capabilities must not be empty")
		assert.Greater(t, m.ContextWindow, 0, "model ContextWindow must be positive")
		assert.Greater(t, m.MaxOutputTokens, 0, "model MaxOutputTokens must be positive")
	}
}

func TestAll_ReturnsCopy(t *testing.T) {
	a := All()
	b := All()
	a[0].ID = "mutated"

	assert.NotEqual(t, "mutated", b[0].ID, "All() must return independent copies")
}

func TestNoDuplicateIDs(t *testing.T) {
	seen := make(map[string]bool, len(registry))

	for _, m := range registry {
		assert.False(t, seen[m.ID], "duplicate model ID: %s", m.ID)
		seen[m.ID] = true
	}
}

func TestByID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantNil bool
	}{
		{
			name:   "exact match",
			input:  modelIDClaudeOpus46,
			wantID: modelIDClaudeOpus46,
		},
		{
			name:   "exact haiku match",
			input:  modelIDClaudeHaiku45,
			wantID: modelIDClaudeHaiku45,
		},
		{
			name:    "not found",
			input:   "gpt-4",
			wantNil: true,
		},
		{
			name:    "short alias removed",
			input:   "opus",
			wantNil: true,
		},
		{
			name:    "dated variant removed",
			input:   modelIDClaudeOpus46 + "-20260205",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ByID(tt.input)
			if tt.wantNil {
				assert.Nil(t, got)

				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.wantID, got.ID)
		})
	}
}

func TestByCostTier(t *testing.T) {
	tests := []struct {
		name    string
		tier    CostTier
		wantMin int
	}{
		{name: "high tier", tier: CostTierHigh, wantMin: 1},
		{name: "medium tier", tier: CostTierMedium, wantMin: 1},
		{name: "low tier", tier: CostTierLow, wantMin: 1},
		{name: "unknown tier", tier: CostTier("unknown"), wantMin: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ByCostTier(tt.tier)
			assert.GreaterOrEqual(t, len(got), tt.wantMin)

			for _, m := range got {
				assert.Equal(t, tt.tier, m.CostTier)
			}
		})
	}
}

func TestCapabilities(t *testing.T) {
	t.Run("known model", func(t *testing.T) {
		caps := Capabilities(modelIDClaudeOpus46)
		require.NotNil(t, caps)
		assert.Contains(t, caps, "vision")
		assert.Contains(t, caps, "tool-use")
		assert.Contains(t, caps, "reasoning")
		assert.Contains(t, caps, "structured-output")
	})

	t.Run("unknown model", func(t *testing.T) {
		caps := Capabilities("nonexistent")
		assert.Nil(t, caps)
	})

	t.Run("removed alias lookup", func(t *testing.T) {
		caps := Capabilities("opus")
		assert.Nil(t, caps)
	})
}

func TestHasCapability(t *testing.T) {
	m := ByID(modelIDClaudeOpus46)
	require.NotNil(t, m)

	assert.True(t, m.HasCapability(CapVision))
	assert.True(t, m.HasCapability(CapToolUse))
	assert.True(t, m.HasCapability(CapReasoning))
	assert.True(t, m.HasCapability(CapStructuredOutput))
	assert.False(t, m.HasCapability(Capability("nonexistent")))
}

func TestCapabilityStrings(t *testing.T) {
	m := ByID(modelIDClaudeOpus46)
	require.NotNil(t, m)

	strs := m.CapabilityStrings()
	assert.Equal(t, []string{"vision", "tool-use", "reasoning", "structured-output"}, strs)
}
