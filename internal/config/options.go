package config

import (
	"log/slog"
	"time"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/hook"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/mcp"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/permission"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/sandbox"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/userinput"
)

// Effort controls thinking depth.
type Effort string

const (
	// EffortLow uses minimal thinking.
	EffortLow Effort = "low"
	// EffortMedium uses moderate thinking.
	EffortMedium Effort = "medium"
	// EffortHigh uses deep thinking.
	EffortHigh Effort = "high"
	// EffortMax uses maximum thinking depth.
	EffortMax Effort = "max"
)

// ThinkingConfig controls extended thinking behavior.
// Implementations: ThinkingConfigAdaptive, ThinkingConfigEnabled, ThinkingConfigDisabled.
type ThinkingConfig interface {
	thinkingConfig() // marker method
}

// ThinkingConfigAdaptive enables adaptive thinking mode.
// Uses a default budget of 32,000 tokens.
type ThinkingConfigAdaptive struct{}

func (ThinkingConfigAdaptive) thinkingConfig() {}

// ThinkingConfigEnabled enables thinking with a specific token budget.
type ThinkingConfigEnabled struct {
	BudgetTokens int
}

func (ThinkingConfigEnabled) thinkingConfig() {}

// ThinkingConfigDisabled disables extended thinking.
type ThinkingConfigDisabled struct{}

func (ThinkingConfigDisabled) thinkingConfig() {}

// Options configures the behavior of the Claude agent.
type Options struct {
	// Logger is the slog logger for debug output.
	// If nil, logging is disabled (silent operation).
	Logger *slog.Logger

	// SystemPrompt is the system message to send to Claude.
	// Use this for a simple string system prompt.
	SystemPrompt string

	// SystemPromptPreset specifies a preset system prompt configuration.
	// If set, this takes precedence over SystemPrompt.
	SystemPromptPreset *SystemPromptPreset

	// Model specifies which Claude model to use (e.g., "claude-3-5-sonnet-20241022")
	Model string

	// PermissionMode controls how permissions are handled
	// Valid values: "acceptEdits", "bypassPermissions", "default", "dontAsk", "plan"
	// Legacy aliases are supported and normalized:
	// - "acceptAll" -> "bypassPermissions"
	// - "prompt" -> "default"
	PermissionMode string

	// MaxTurns limits the maximum number of conversation turns
	MaxTurns int

	// Cwd sets the working directory for the CLI process
	Cwd string

	// ClaudeHome overrides the Claude home directory (default: ~/.claude).
	ClaudeHome string

	// CliPath is the explicit path to the claude CLI binary
	// If empty, the CLI will be searched in PATH
	CliPath string

	// Env provides additional environment variables for the CLI process
	Env map[string]string

	// Hooks configures event hooks for tool interception
	Hooks map[hook.Event][]*hook.Matcher

	// Thinking controls extended thinking behavior.
	Thinking ThinkingConfig

	// Effort controls thinking depth.
	// If nil, no effort flag is passed to the CLI.
	Effort *Effort

	// IncludePartialMessages enables streaming of partial message updates.
	IncludePartialMessages bool

	// MaxBudgetUSD sets a cost limit for the session in USD.
	// If nil, no budget limit is imposed.
	MaxBudgetUSD *float64

	// MCPServers configures external MCP servers to connect to.
	// Map key is the server name, value is the server configuration.
	// Use this for programmatic configuration.
	MCPServers map[string]mcp.ServerConfig

	// MCPConfig is a path to an MCP config file or a raw JSON string.
	// If set, this takes precedence over MCPServers.
	// Use this for file-based configuration or pre-formatted JSON.
	MCPConfig string

	// CanUseTool is called before each tool use for permission checking.
	// If nil, all tool uses are allowed.
	CanUseTool permission.Callback

	// OnUserInput handles AskUserQuestion tool prompts.
	// If nil, AskUserQuestion is handled by normal tool permission flow.
	OnUserInput userinput.Callback

	// SandboxSettings configures CLI sandbox behavior.
	// If nil, sandbox is not enabled.
	SandboxSettings *sandbox.Settings

	// ===== NEW FIELDS FOR 1:1 PARITY =====

	// Tools specifies which tools are available. Accepts one of:
	// - ToolsList: Array of tool names (e.g., ToolsList{"Read", "Glob", "Grep"})
	// - *ToolsPreset: Preset configuration (e.g., &ToolsPreset{Type: "preset", Preset: "claude_code"})
	// If nil, all default tools are available.
	// Can be combined with AllowedTools/DisallowedTools for fine-grained control.
	Tools ToolsConfig

	// AllowedTools is a list of pre-approved tools that can be used without prompting.
	// These tools will skip permission confirmation dialogs.
	AllowedTools []string

	// DisallowedTools is a list of tools that are explicitly blocked.
	DisallowedTools []string

	// FallbackModel specifies a model to use if the primary model fails.
	FallbackModel string

	// Betas is a list of beta features to enable.
	Betas []Beta

	// PermissionPromptToolName specifies the tool name to use for permission prompts.
	PermissionPromptToolName string

	// Settings is the path to a settings file to load.
	Settings string

	// AddDirs is a list of additional directories to make accessible.
	AddDirs []string

	// ExtraArgs provides arbitrary CLI flags to pass to the CLI.
	// If the value is nil, the flag is passed without a value (boolean flag).
	ExtraArgs map[string]*string

	// MaxBufferSize sets the maximum bytes for CLI stdout buffering.
	// If nil, uses default buffering.
	MaxBufferSize *int

	// Stderr is a callback function for handling stderr output.
	Stderr func(string)

	// User is a user identifier for tracking purposes.
	User string

	// ContinueConversation indicates whether to continue an existing conversation.
	ContinueConversation bool

	// Resume is a session ID to resume from.
	Resume string

	// ForkSession indicates whether to fork the resumed session to a new ID.
	ForkSession bool

	// Agents defines custom agent configurations.
	// Map key is the agent name.
	Agents map[string]*AgentDefinition

	// SettingSources specifies which setting sources to use.
	SettingSources []SettingSource

	// Plugins is a list of plugin configurations to load.
	Plugins []*PluginConfig

	// OutputFormat specifies a JSON schema for structured output.
	// Accepts either the wrapped format {"type": "json_schema", "schema": {...}}
	// or a raw JSON schema {"type": "object", "properties": {...}} which is
	// auto-detected and used directly.
	OutputFormat map[string]any

	// EnableFileCheckpointing enables file change tracking and rewinding.
	EnableFileCheckpointing bool

	// InitializeTimeout is the timeout for the initialize control request.
	// If nil, defaults to 60 seconds. Can also be set via CLAUDE_CODE_STREAM_CLOSE_TIMEOUT env var.
	InitializeTimeout *time.Duration

	// Transport allows injecting a custom transport implementation.
	// If nil, the default CLITransport is created automatically.
	// This field is not serialized to JSON.
	Transport Transport `json:"-"`
}
