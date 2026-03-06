package models

// Info describes a Claude model in a Codex-like model listing payload.
type Info struct {
	ID                        string                  `json:"id"`
	Model                     string                  `json:"model"`
	DisplayName               string                  `json:"displayName"`
	Description               string                  `json:"description"`
	IsDefault                 bool                    `json:"isDefault"`
	Hidden                    bool                    `json:"hidden"`
	DefaultReasoningEffort    string                  `json:"defaultReasoningEffort"`
	SupportedReasoningEfforts []ReasoningEffortOption `json:"supportedReasoningEfforts"`
	InputModalities           []string                `json:"inputModalities"`
	SupportsPersonality       bool                    `json:"supportsPersonality"`
	Metadata                  map[string]any          `json:"metadata,omitempty"`
}

// ReasoningEffortOption describes a selectable reasoning effort level.
type ReasoningEffortOption struct {
	Value string `json:"reasoningEffort"`
	Label string `json:"description"`
}

// ListResponse is the static response payload backing ListModels.
type ListResponse struct {
	Models   []Info         `json:"models"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

var supportedAdaptiveReasoningEfforts = []ReasoningEffortOption{
	{Value: "low", Label: "Fast responses with lighter reasoning"},
	{Value: "medium", Label: "Balanced reasoning depth for everyday coding tasks"},
	{Value: "high", Label: "Greater reasoning depth for complex planning and implementation"},
}

var adaptiveReasoningModels = map[string]bool{
	modelIDClaudeOpus46:   true,
	modelIDClaudeSonnet46: true,
}

// List returns the static Claude model list in the Codex-like payload shape.
func List() []Info {
	out := make([]Info, 0, len(registry))

	for _, model := range registry {
		info := Info{
			ID:                        model.ID,
			Model:                     model.ID,
			DisplayName:               model.Name,
			Description:               describeModel(model),
			IsDefault:                 false,
			Hidden:                    false,
			DefaultReasoningEffort:    "",
			SupportedReasoningEfforts: []ReasoningEffortOption{},
			InputModalities:           inputModalities(model),
			SupportsPersonality:       false,
			Metadata: map[string]any{
				"source":             "static-catalog",
				"kind":               "model",
				"acceptedByCLI":      true,
				"modelContextWindow": model.ContextWindow,
				"maxOutputTokens":    model.MaxOutputTokens,
			},
		}

		if adaptiveReasoningModels[model.ID] {
			info.DefaultReasoningEffort = "medium"
			info.SupportedReasoningEfforts = cloneReasoningEfforts(
				supportedAdaptiveReasoningEfforts,
			)
		}

		out = append(out, info)
	}

	return out
}

// Response returns the full static model list response payload.
func Response() ListResponse {
	return ListResponse{
		Models: List(),
		Metadata: map[string]any{
			"source":                "static-catalog",
			"acceptedByCLI":         true,
			"supportsLiveDiscovery": false,
		},
	}
}

func inputModalities(model Model) []string {
	modalities := []string{"text"}
	if model.HasCapability(CapVision) {
		modalities = append(modalities, "image")
	}

	return modalities
}

func describeModel(model Model) string {
	switch model.ID {
	case modelIDClaudeOpus46:
		return "Latest Opus model for deep reasoning, planning, and complex coding work."
	case modelIDClaudeSonnet46:
		return "Latest Sonnet model for everyday coding and agent workflows."
	case modelIDClaudeHaiku45:
		return "Fast and cost-efficient Claude model for lighter coding and analysis tasks."
	default:
		return model.Name
	}
}

func cloneReasoningEfforts(src []ReasoningEffortOption) []ReasoningEffortOption {
	if src == nil {
		return nil
	}

	if len(src) == 0 {
		return []ReasoningEffortOption{}
	}

	out := make([]ReasoningEffortOption, len(src))
	copy(out, src)

	return out
}
