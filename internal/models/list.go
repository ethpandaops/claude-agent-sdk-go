package models

import "encoding/json"

// Info describes a Claude model or accepted Claude CLI alias in a Codex-like
// model listing payload.
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
	Value string `json:"-"`
	Label string `json:"-"`
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
	modelIDOpus1M:         true,
	modelIDSonnet1M:       true,
}

var hiddenModels = map[string]bool{
	"claude-opus-4-5":   true,
	"claude-sonnet-4-5": true,
	"claude-opus-4-1":   true,
	"claude-opus-4-0":   true,
	"claude-sonnet-4-0": true,
}

var aliasEntries = []Info{
	{
		ID:          modelIDDefault,
		Model:       modelIDDefault,
		DisplayName: "Claude Code Default",
		Description: "Claude Code runtime default alias. Resolves to the default model for the current account and session mode.",
		IsDefault:   true,
		Hidden:      false,
		InputModalities: []string{
			"text",
			"image",
		},
		SupportsPersonality:       false,
		SupportedReasoningEfforts: []ReasoningEffortOption{},
		Metadata: map[string]any{
			"source":              "static-alias",
			"kind":                "alias",
			"resolvesDynamically": true,
			"acceptedByCLI":       true,
		},
	},
	{
		ID:                     "opusplan",
		Model:                  "opusplan",
		DisplayName:            "Claude Code OpusPlan",
		Description:            "Claude Code alias for plan-focused Opus routing.",
		IsDefault:              false,
		Hidden:                 false,
		DefaultReasoningEffort: "medium",
		SupportedReasoningEfforts: cloneReasoningEfforts(
			supportedAdaptiveReasoningEfforts,
		),
		InputModalities: []string{
			"text",
			"image",
		},
		SupportsPersonality: false,
		Metadata: map[string]any{
			"source":             "static-alias",
			"kind":               "alias",
			"acceptedByCLI":      true,
			"resolvesTo":         modelIDClaudeOpus46,
			"modelContextWindow": 200000,
			"maxOutputTokens":    128000,
		},
	},
}

// List returns the static Claude model list in the Codex-like payload shape.
func List() []Info {
	out := make([]Info, 0, len(aliasEntries)+len(registry))

	for _, alias := range aliasEntries {
		out = append(out, cloneInfo(alias))
	}

	for _, model := range registry {
		info := Info{
			ID:                        model.ID,
			Model:                     model.ID,
			DisplayName:               model.Name,
			Description:               describeModel(model),
			IsDefault:                 false,
			Hidden:                    hiddenModels[model.ID],
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

		if len(model.Aliases) > 0 {
			info.Metadata["aliases"] = append([]string(nil), model.Aliases...)
		}

		switch model.ID {
		case modelIDSonnet1M:
			info.Metadata["kind"] = "variant-alias"
			info.Metadata["resolvesTo"] = modelIDClaudeSonnet46
		case modelIDOpus1M:
			info.Metadata["kind"] = "variant-alias"
			info.Metadata["resolvesTo"] = modelIDClaudeOpus46
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
	case modelIDSonnet1M:
		return "Claude Sonnet 4.6 with the 1M context window variant."
	case "claude-haiku-4-5":
		return "Fast and cost-efficient Claude model for lighter coding and analysis tasks."
	case "claude-opus-4-5":
		return "Previous Opus generation retained for pinned workflows."
	case "claude-sonnet-4-5":
		return "Previous Sonnet generation retained for pinned workflows."
	case "claude-opus-4-1":
		return "Legacy Opus model retained for pinned compatibility."
	case "claude-opus-4-0":
		return "Legacy Claude Opus 4 model retained for pinned compatibility."
	case "claude-sonnet-4-0":
		return "Legacy Claude Sonnet 4 model retained for pinned compatibility."
	case modelIDOpus1M:
		return "Claude Opus 4.6 with the 1M context window variant."
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

func cloneInfo(src Info) Info {
	dst := src

	if src.InputModalities != nil {
		dst.InputModalities = append([]string(nil), src.InputModalities...)
	}

	if src.SupportedReasoningEfforts != nil {
		dst.SupportedReasoningEfforts = cloneReasoningEfforts(src.SupportedReasoningEfforts)
	}

	if src.Metadata != nil {
		dst.Metadata = cloneMetadata(src.Metadata)
	}

	return dst
}

func cloneMetadata(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for key, value := range src {
		switch typed := value.(type) {
		case []string:
			dst[key] = append([]string(nil), typed...)
		default:
			dst[key] = typed
		}
	}

	return dst
}

// UnmarshalJSON accepts both the legacy SDK field names and the current
// Codex-like payload shape.
func (o *ReasoningEffortOption) UnmarshalJSON(data []byte) error {
	var raw struct {
		ReasoningEffort string `json:"reasoningEffort"`
		Description     string `json:"description"`
		Value           string `json:"value"`
		Label           string `json:"label"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.ReasoningEffort != "" {
		o.Value = raw.ReasoningEffort
	} else {
		o.Value = raw.Value
	}

	if raw.Description != "" {
		o.Label = raw.Description
	} else {
		o.Label = raw.Label
	}

	return nil
}

// MarshalJSON emits the Codex-like field names used by this SDK payload.
func (o ReasoningEffortOption) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ReasoningEffort string `json:"reasoningEffort"`
		Description     string `json:"description"`
	}{
		ReasoningEffort: o.Value,
		Description:     o.Label,
	})
}
