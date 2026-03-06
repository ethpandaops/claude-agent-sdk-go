package claudesdk

import (
	"context"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/models"
)

// Public model types.

// Model holds metadata for a single Claude model.
type Model = models.Model

// ModelInfo describes a model exposed by ListModels.
type ModelInfo = models.Info

// ReasoningEffortOption describes a selectable reasoning effort level.
type ReasoningEffortOption = models.ReasoningEffortOption

// ModelListResponse is the response payload backing ListModels.
type ModelListResponse = models.ListResponse

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

// Models returns a copy of the SDK's static Claude model catalog.
// It is not a live list of models available to the logged-in Claude CLI user.
func Models() []Model {
	return models.All()
}

// ListModels returns a Codex-like model list payload backed by the SDK's static
// Claude catalog. This is not a live account-specific list from the Claude CLI.
func ListModels(_ context.Context) ([]ModelInfo, error) {
	return models.List(), nil
}

// ListModelsResponse returns the full static model-list response payload.
func ListModelsResponse(_ context.Context) (*ModelListResponse, error) {
	resp := models.Response()

	return &resp, nil
}

// ModelByID looks up a model by exact ID.
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
