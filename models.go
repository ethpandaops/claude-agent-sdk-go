package claudesdk

import "github.com/ethpandaops/claude-agent-sdk-go/internal/models"

// Re-export model types from internal/models.

// Model holds metadata for a single Claude model.
type Model = models.Model

// ModelCapability represents a model capability such as vision or tool use.
type ModelCapability = models.Capability

// ModelCostTier represents a provider-agnostic relative cost tier.
type ModelCostTier = models.CostTier

// Model capability constants.
const (
	// ModelCapVision indicates the model supports image/vision inputs.
	ModelCapVision = models.CapVision
	// ModelCapToolUse indicates the model supports tool/function calling.
	ModelCapToolUse = models.CapToolUse
	// ModelCapReasoning indicates the model supports extended reasoning.
	ModelCapReasoning = models.CapReasoning
	// ModelCapStructuredOutput indicates the model supports structured JSON output.
	ModelCapStructuredOutput = models.CapStructuredOutput
)

// Model cost tier constants.
const (
	// ModelCostTierHigh represents opus-class pricing.
	ModelCostTierHigh = models.CostTierHigh
	// ModelCostTierMedium represents sonnet-class pricing.
	ModelCostTierMedium = models.CostTierMedium
	// ModelCostTierLow represents haiku-class pricing.
	ModelCostTierLow = models.CostTierLow
)

// Models returns a copy of all known Claude models.
func Models() []Model {
	return models.All()
}

// ModelByID looks up a model by ID, alias, or dated prefix.
// Returns nil if no model is found.
func ModelByID(id string) *Model {
	return models.ByID(id)
}

// ModelsByCostTier returns all models matching the given cost tier.
func ModelsByCostTier(tier ModelCostTier) []Model {
	return models.ByCostTier(tier)
}

// ModelCapabilities returns capability strings for the given model ID.
// Returns nil if the model is not found.
func ModelCapabilities(modelID string) []string {
	return models.Capabilities(modelID)
}
