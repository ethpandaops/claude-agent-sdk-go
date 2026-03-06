package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList_IncludesCurrentConcreteEntries(t *testing.T) {
	t.Parallel()

	models := List()
	require.NotEmpty(t, models)
	require.Len(t, models, 3)

	var (
		haiku45  Info
		sonnet46 Info
		opus46   Info
	)

	for _, model := range models {
		switch model.ID {
		case modelIDClaudeHaiku45:
			haiku45 = model
		case modelIDClaudeSonnet46:
			sonnet46 = model
		case modelIDClaudeOpus46:
			opus46 = model
		}
	}

	require.Equal(t, modelIDClaudeHaiku45, haiku45.ID)
	assert.False(t, haiku45.IsDefault)
	assert.Equal(t, 200000, haiku45.Metadata["modelContextWindow"])
	assert.Equal(t, 64_000, haiku45.Metadata["maxOutputTokens"])

	require.Equal(t, modelIDClaudeSonnet46, sonnet46.ID)
	assert.Equal(t, "medium", sonnet46.DefaultReasoningEffort)
	require.Len(t, sonnet46.SupportedReasoningEfforts, 3)

	require.Equal(t, modelIDClaudeOpus46, opus46.ID)
	assert.Equal(t, 200000, opus46.Metadata["modelContextWindow"])
	assert.Equal(t, 128000, opus46.Metadata["maxOutputTokens"])
}

func TestResponse_Metadata(t *testing.T) {
	t.Parallel()

	resp := Response()
	require.NotEmpty(t, resp.Models)
	assert.Equal(t, map[string]any{
		"source":                "static-catalog",
		"acceptedByCLI":         true,
		"supportsLiveDiscovery": false,
	}, resp.Metadata)
}
