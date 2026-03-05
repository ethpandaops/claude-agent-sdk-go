package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
)

// Command represents the CLI command to execute.
type Command struct {
	// Args are the command line arguments.
	Args []string

	// Env are the environment variables.
	Env []string
}

// BuildArgs constructs the CLI command arguments.
//
// When isStreaming is true, uses --input-format stream-json and omits the prompt
// from command line arguments (prompt comes via stdin instead).
//
//nolint:gocyclo // High complexity is acceptable here as each branch independently adds CLI flags
func BuildArgs(
	prompt string,
	options *config.Options,
	isStreaming bool,
) []string {
	// Start with output format and verbose
	args := []string{
		"--output-format", "stream-json",
		"--verbose",
	}

	// Add optional configuration flags
	if options.PermissionMode != "" {
		args = append(args, "--permission-mode", config.NormalizePermissionMode(options.PermissionMode))
	}

	if options.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(options.MaxTurns))
	}

	if options.Model != "" {
		args = append(args, "--model", options.Model)
	}

	// System prompt - always set this flag (empty string when not provided)
	// For presets with Append field, use --append-system-prompt instead
	if options.SystemPromptPreset != nil {
		if options.SystemPromptPreset.Append != nil && *options.SystemPromptPreset.Append != "" {
			args = append(args, "--append-system-prompt", *options.SystemPromptPreset.Append)
		}
		// Don't serialize full preset to --system-prompt, just use the append
	} else if options.SystemPrompt != "" {
		args = append(args, "--system-prompt", options.SystemPrompt)
	} else {
		args = append(args, "--system-prompt", "")
	}

	// Thinking configuration
	if options.Thinking != nil {
		switch t := options.Thinking.(type) {
		case config.ThinkingConfigAdaptive:
			args = append(args, "--max-thinking-tokens", "32000")
		case config.ThinkingConfigEnabled:
			args = append(args, "--max-thinking-tokens", strconv.Itoa(t.BudgetTokens))
		case config.ThinkingConfigDisabled:
			args = append(args, "--max-thinking-tokens", "0")
		}
	}

	// Effort flag
	if options.Effort != nil {
		args = append(args, "--effort", string(*options.Effort))
	}

	if options.IncludePartialMessages {
		args = append(args, "--include-partial-messages")
	}

	if options.MaxBudgetUSD != nil {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%g", *options.MaxBudgetUSD))
	}

	// MCP configuration via --mcp-config flag
	// MCPConfig takes precedence over MCPServers
	if options.MCPConfig != "" {
		// Direct file path or JSON string - pass as-is
		args = append(args, "--mcp-config", options.MCPConfig)
	}

	if options.MCPConfig == "" && len(options.MCPServers) > 0 {
		// Build JSON config with mcpServers wrapper
		config := map[string]any{"mcpServers": options.MCPServers}

		configJSON, err := json.Marshal(config)
		if err == nil {
			args = append(args, "--mcp-config", string(configJSON))
		}
	}

	// Settings with sandbox merged
	// Note: sandbox is merged into --settings JSON, not passed as separate --sandbox flag
	settingsValue := buildSettingsValue(options)
	if settingsValue != "" {
		args = append(args, "--settings", settingsValue)
	}

	// ===== TOOL CONFIGURATION =====

	// Tools field - maps to --tools flag (base set of available tools)
	if options.Tools != nil {
		switch t := options.Tools.(type) {
		case config.ToolsList:
			if len(t) == 0 {
				args = append(args, "--tools", "")
			} else {
				args = append(args, "--tools", strings.Join(t, ","))
			}
		case *config.ToolsPreset:
			// Preset object - 'claude_code' preset maps to 'default'
			args = append(args, "--tools", "default")
		}
	}

	// AllowedTools - pre-approve tools to skip permission prompts
	if len(options.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(options.AllowedTools, ","))
	}

	// DisallowedTools - block specific tools
	if len(options.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(options.DisallowedTools, ","))
	}

	// Fallback model
	if options.FallbackModel != "" {
		args = append(args, "--fallback-model", options.FallbackModel)
	}

	// Betas (comma-separated, single flag)
	if len(options.Betas) > 0 {
		betaStrs := make([]string, len(options.Betas))

		for i, beta := range options.Betas {
			betaStrs[i] = string(beta)
		}

		args = append(args, "--betas", strings.Join(betaStrs, ","))
	}

	// Permission prompt tool name
	if options.PermissionPromptToolName != "" {
		args = append(args, "--permission-prompt-tool", options.PermissionPromptToolName)
	}

	// Note: --settings is handled earlier via buildSettingsValue which merges sandbox settings

	// Additional directories
	for _, dir := range options.AddDirs {
		args = append(args, "--add-dir", dir)
	}

	// Continue conversation
	if options.ContinueConversation {
		args = append(args, "--continue")
	}

	// Resume session
	if options.Resume != "" {
		args = append(args, "--resume", options.Resume)
	}

	// Fork session
	if options.ForkSession {
		args = append(args, "--fork-session")
	}

	// Note: Agents are sent via the initialize control request, not CLI flags.
	// This avoids platform-specific ARG_MAX limits for large agent definitions.

	// Setting sources - always set this flag (can be empty)
	sources := make([]string, len(options.SettingSources))
	for i, s := range options.SettingSources {
		sources[i] = string(s)
	}

	args = append(args, "--setting-sources", strings.Join(sources, ","))

	// Plugins
	for _, plugin := range options.Plugins {
		args = append(args, "--plugin-dir", plugin.Path)
	}

	// Output format (structured output JSON schema)
	if options.OutputFormat != nil {
		if schema := extractJSONSchema(options.OutputFormat); schema != nil {
			schemaJSON, err := json.Marshal(schema)
			if err == nil {
				args = append(args, "--json-schema", string(schemaJSON))
			}
		}
	}

	// Note: --user is not a CLI flag
	// Note: --max-buffer-size is not a CLI flag
	// MaxBufferSize is kept in types for internal buffer limit use

	// Extra args (arbitrary CLI flags)
	for key, value := range options.ExtraArgs {
		if value == nil {
			// Boolean flag without value
			args = append(args, "--"+key)
		} else {
			// Flag with value
			args = append(args, "--"+key, *value)
		}
	}

	// Handle prompt based on mode
	if isStreaming {
		// Streaming mode: use --input-format stream-json, prompt comes via stdin
		args = append(args, "--input-format", "stream-json")
	} else {
		// One-shot mode: use --print with the prompt after --
		args = append(args, "--print", "--", prompt)
	}

	return args
}

// buildSettingsValue constructs the --settings CLI argument value.
// It merges sandbox settings into the settings JSON object.
func buildSettingsValue(options *config.Options) string {
	hasSettings := options.Settings != ""
	hasSandbox := options.SandboxSettings != nil

	if !hasSettings && !hasSandbox {
		return ""
	}

	// If only settings path and no sandbox, pass through as-is
	if hasSettings && !hasSandbox {
		return options.Settings
	}

	// If we have sandbox settings, merge into JSON object
	settingsObj := make(map[string]any, 2)

	if hasSettings {
		// Check if settings is JSON string or file path
		settingsStr := strings.TrimSpace(options.Settings)
		if strings.HasPrefix(settingsStr, "{") && strings.HasSuffix(settingsStr, "}") {
			// Parse existing JSON settings
			_ = json.Unmarshal([]byte(settingsStr), &settingsObj)
		}
		// Note: file path reading would need to be added for file paths if needed
	}

	if hasSandbox {
		settingsObj["sandbox"] = options.SandboxSettings
	}

	result, err := json.Marshal(settingsObj)
	if err != nil {
		return ""
	}

	return string(result)
}

// extractJSONSchema extracts the inner JSON schema from an OutputFormat map.
// It supports two formats:
//   - Wrapped: {"type": "json_schema", "schema": {...}} — returns the inner schema.
//   - Raw: {"type": "object", "properties": {...}} — returns the map as-is (auto-wrap).
//
// Returns nil if the map doesn't match either format.
func extractJSONSchema(outputFormat map[string]any) map[string]any {
	formatType, _ := outputFormat["type"].(string)

	// Wrapped format: {"type": "json_schema", "schema": {...}}
	if formatType == "json_schema" {
		if schema, ok := outputFormat["schema"].(map[string]any); ok {
			return schema
		}

		return nil
	}

	// Raw schema: has "properties" key (e.g. {"type": "object", "properties": {...}})
	if _, hasProperties := outputFormat["properties"]; hasProperties {
		return outputFormat
	}

	return nil
}

// BuildEnvironment constructs the environment variables for the CLI process.
func BuildEnvironment(options *config.Options) []string {
	// Start with current environment
	env := os.Environ()

	// Add SDK-specific environment variables
	env = append(env, "CLAUDE_AGENT_SDK_VERSION=0.1.0")
	env = append(env, "CLAUDE_CODE_ENTRYPOINT=sdk-go")

	// File checkpointing
	if options.EnableFileCheckpointing {
		env = append(env, "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=true")
	}

	// Add or override with user-provided environment variables
	for key, value := range options.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}
