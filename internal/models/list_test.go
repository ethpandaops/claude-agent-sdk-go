package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList_IncludesAliasAndConcreteEntries(t *testing.T) {
	t.Parallel()

	models := List()
	require.NotEmpty(t, models)

	var (
		defaultModel Info
		sonnet1M     Info
		opus46       Info
	)

	for _, model := range models {
		switch model.ID {
		case modelIDDefault:
			defaultModel = model
		case modelIDSonnet1M:
			sonnet1M = model
		case modelIDClaudeOpus46:
			opus46 = model
		}
	}

	require.Equal(t, modelIDDefault, defaultModel.ID)
	assert.True(t, defaultModel.IsDefault)
	assert.Equal(t, "static-alias", defaultModel.Metadata["source"])

	require.Equal(t, modelIDSonnet1M, sonnet1M.ID)
	assert.Equal(t, 1000000, sonnet1M.Metadata["modelContextWindow"])
	assert.Equal(t, modelIDClaudeSonnet46, sonnet1M.Metadata["resolvesTo"])
	assert.Equal(t, "medium", sonnet1M.DefaultReasoningEffort)
	require.Len(t, sonnet1M.SupportedReasoningEfforts, 3)

	require.Equal(t, modelIDClaudeOpus46, opus46.ID)
	assert.Equal(t, 200000, opus46.Metadata["modelContextWindow"])
	assert.Equal(t, 128000, opus46.Metadata["maxOutputTokens"])
	assert.Equal(t, []string{"opus"}, opus46.Metadata["aliases"])
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
