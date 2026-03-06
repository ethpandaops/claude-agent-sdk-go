package claudesdk

import (
	"log/slog"
	"time"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
)

// Option configures ClaudeAgentOptions using the functional options pattern.
// This is the primary option type for configuring clients and queries.
type Option func(*ClaudeAgentOptions)

// applyAgentOptions applies functional options to a ClaudeAgentOptions struct.
func applyAgentOptions(opts []Option) *ClaudeAgentOptions {
	options := &ClaudeAgentOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return options
}

// ===== Basic Configuration =====

// WithLogger sets the logger for debug output.
// If not set, logging is disabled (silent operation).
func WithLogger(logger *slog.Logger) Option {
	return func(o *ClaudeAgentOptions) {
		o.Logger = logger
	}
}

// WithSystemPrompt sets the system message to send to Claude.
func WithSystemPrompt(prompt string) Option {
	return func(o *ClaudeAgentOptions) {
		o.SystemPrompt = prompt
	}
}

// WithSystemPromptPreset sets a preset system prompt configuration.
// If set, this takes precedence over WithSystemPrompt.
func WithSystemPromptPreset(preset *SystemPromptPreset) Option {
	return func(o *ClaudeAgentOptions) {
		o.SystemPromptPreset = preset
	}
}

// WithModel specifies which Claude model or alias to use
// (e.g., "claude-sonnet-4-6", "sonnet", or "sonnet[1m]").
func WithModel(model string) Option {
	return func(o *ClaudeAgentOptions) {
		o.Model = model
	}
}

// WithPermissionMode controls how permissions are handled.
// Valid values: "default", "acceptEdits", "plan", "bypassPermissions".
func WithPermissionMode(mode string) Option {
	return func(o *ClaudeAgentOptions) {
		o.PermissionMode = mode
	}
}

// WithMaxTurns limits the maximum number of conversation turns.
func WithMaxTurns(maxTurns int) Option {
	return func(o *ClaudeAgentOptions) {
		o.MaxTurns = maxTurns
	}
}

// WithCwd sets the working directory for the CLI process.
func WithCwd(cwd string) Option {
	return func(o *ClaudeAgentOptions) {
		o.Cwd = cwd
	}
}

// WithClaudeHome overrides the Claude home directory (default: ~/.claude).
func WithClaudeHome(path string) Option {
	return func(o *ClaudeAgentOptions) {
		o.ClaudeHome = path
	}
}

// WithCliPath sets the explicit path to the claude CLI binary.
// If not set, the CLI will be searched in PATH.
func WithCliPath(path string) Option {
	return func(o *ClaudeAgentOptions) {
		o.CliPath = path
	}
}

// WithEnv provides additional environment variables for the CLI process.
func WithEnv(env map[string]string) Option {
	return func(o *ClaudeAgentOptions) {
		o.Env = env
	}
}

// WithUser sets a user identifier for tracking purposes.
func WithUser(user string) Option {
	return func(o *ClaudeAgentOptions) {
		o.User = user
	}
}

// ===== Hooks =====

// WithHooks configures event hooks for tool interception.
func WithHooks(hooks map[HookEvent][]*HookMatcher) Option {
	return func(o *ClaudeAgentOptions) {
		o.Hooks = hooks
	}
}

// ===== Token/Budget =====

// WithThinking sets the thinking configuration.
func WithThinking(thinking config.ThinkingConfig) Option {
	return func(o *ClaudeAgentOptions) {
		o.Thinking = thinking
	}
}

// WithEffort sets the thinking effort level.
func WithEffort(effort config.Effort) Option {
	return func(o *ClaudeAgentOptions) {
		o.Effort = &effort
	}
}

// WithIncludePartialMessages enables streaming of partial message updates.
func WithIncludePartialMessages(include bool) Option {
	return func(o *ClaudeAgentOptions) {
		o.IncludePartialMessages = include
	}
}

// WithMaxBudgetUSD sets a cost limit for the session in USD.
func WithMaxBudgetUSD(budget float64) Option {
	return func(o *ClaudeAgentOptions) {
		o.MaxBudgetUSD = &budget
	}
}

// WithMaxBufferSize sets the maximum bytes for CLI stdout buffering.
func WithMaxBufferSize(size int) Option {
	return func(o *ClaudeAgentOptions) {
		o.MaxBufferSize = &size
	}
}

// ===== MCP =====

// WithMCPServers configures external MCP servers to connect to.
// Map key is the server name, value is the server configuration.
func WithMCPServers(servers map[string]MCPServerConfig) Option {
	return func(o *ClaudeAgentOptions) {
		o.MCPServers = servers
	}
}

// WithMCPConfig sets a path to an MCP config file or a raw JSON string.
// If set, this takes precedence over WithMCPServers.
func WithMCPConfig(config string) Option {
	return func(o *ClaudeAgentOptions) {
		o.MCPConfig = config
	}
}

// ===== Tools =====

// WithTools specifies which tools are available.
// Accepts ToolsList (tool names) or *ToolsPreset.
func WithTools(tools config.ToolsConfig) Option {
	return func(o *ClaudeAgentOptions) {
		o.Tools = tools
	}
}

// WithAllowedTools sets pre-approved tools that can be used without prompting.
func WithAllowedTools(tools ...string) Option {
	return func(o *ClaudeAgentOptions) {
		o.AllowedTools = tools
	}
}

// WithDisallowedTools sets tools that are explicitly blocked.
func WithDisallowedTools(tools ...string) Option {
	return func(o *ClaudeAgentOptions) {
		o.DisallowedTools = tools
	}
}

// WithCanUseTool sets a callback for permission checking before each tool use.
func WithCanUseTool(callback ToolPermissionCallback) Option {
	return func(o *ClaudeAgentOptions) {
		o.CanUseTool = callback
	}
}

// WithOnUserInput sets a callback for handling AskUserQuestion prompts.
func WithOnUserInput(callback UserInputCallback) Option {
	return func(o *ClaudeAgentOptions) {
		o.OnUserInput = callback
	}
}

// ===== Session =====

// WithContinueConversation indicates whether to continue an existing conversation.
func WithContinueConversation(cont bool) Option {
	return func(o *ClaudeAgentOptions) {
		o.ContinueConversation = cont
	}
}

// WithResume sets a session ID to resume from.
func WithResume(sessionID string) Option {
	return func(o *ClaudeAgentOptions) {
		o.Resume = sessionID
	}
}

// WithForkSession indicates whether to fork the resumed session to a new ID.
func WithForkSession(fork bool) Option {
	return func(o *ClaudeAgentOptions) {
		o.ForkSession = fork
	}
}

// ===== Advanced =====

// WithFallbackModel specifies a model to use if the primary model fails.
func WithFallbackModel(model string) Option {
	return func(o *ClaudeAgentOptions) {
		o.FallbackModel = model
	}
}

// WithBetas enables beta features.
func WithBetas(betas ...SdkBeta) Option {
	return func(o *ClaudeAgentOptions) {
		o.Betas = betas
	}
}

// WithPermissionPromptToolName specifies the tool name to use for permission prompts.
func WithPermissionPromptToolName(name string) Option {
	return func(o *ClaudeAgentOptions) {
		o.PermissionPromptToolName = name
	}
}

// WithSettings sets the path to a settings file to load.
func WithSettings(path string) Option {
	return func(o *ClaudeAgentOptions) {
		o.Settings = path
	}
}

// WithAddDirs adds additional directories to make accessible.
func WithAddDirs(dirs ...string) Option {
	return func(o *ClaudeAgentOptions) {
		o.AddDirs = dirs
	}
}

// WithExtraArgs provides arbitrary CLI flags to pass to the CLI.
// If the value is nil, the flag is passed without a value (boolean flag).
func WithExtraArgs(args map[string]*string) Option {
	return func(o *ClaudeAgentOptions) {
		o.ExtraArgs = args
	}
}

// WithStderr sets a callback function for handling stderr output.
func WithStderr(handler func(string)) Option {
	return func(o *ClaudeAgentOptions) {
		o.Stderr = handler
	}
}

// WithSandboxSettings configures CLI sandbox behavior.
func WithSandboxSettings(settings *SandboxSettings) Option {
	return func(o *ClaudeAgentOptions) {
		o.SandboxSettings = settings
	}
}

// WithAgents defines custom agent configurations.
func WithAgents(agents map[string]*AgentDefinition) Option {
	return func(o *ClaudeAgentOptions) {
		o.Agents = agents
	}
}

// WithSettingSources specifies which setting sources to use.
func WithSettingSources(sources ...SettingSource) Option {
	return func(o *ClaudeAgentOptions) {
		o.SettingSources = sources
	}
}

// WithPlugins configures plugins to load.
func WithPlugins(plugins ...*SdkPluginConfig) Option {
	return func(o *ClaudeAgentOptions) {
		o.Plugins = plugins
	}
}

// WithOutputFormat specifies a JSON schema for structured output.
//
// The canonical format uses a wrapper object:
//
//	claudesdk.WithOutputFormat(map[string]any{
//	    "type": "json_schema",
//	    "schema": map[string]any{
//	        "type":       "object",
//	        "properties": map[string]any{...},
//	        "required":   []string{...},
//	    },
//	})
//
// Raw JSON schemas (without the wrapper) are also accepted and auto-wrapped:
//
//	claudesdk.WithOutputFormat(map[string]any{
//	    "type":       "object",
//	    "properties": map[string]any{...},
//	    "required":   []string{...},
//	})
//
// Structured output is available on [ResultMessage].StructuredOutput (parsed)
// or [ResultMessage].Result (JSON string).
func WithOutputFormat(format map[string]any) Option {
	return func(o *ClaudeAgentOptions) {
		o.OutputFormat = format
	}
}

// WithEnableFileCheckpointing enables file change tracking and rewinding.
func WithEnableFileCheckpointing(enable bool) Option {
	return func(o *ClaudeAgentOptions) {
		o.EnableFileCheckpointing = enable
	}
}

// WithInitializeTimeout sets the timeout for the initialize control request.
func WithInitializeTimeout(timeout time.Duration) Option {
	return func(o *ClaudeAgentOptions) {
		o.InitializeTimeout = &timeout
	}
}

// WithSDKTools registers high-level Tool instances as an in-process MCP server.
// Tools are exposed under the "sdk" MCP server name (tool names: mcp__sdk__<name>).
// Each tool is automatically added to AllowedTools.
func WithSDKTools(tools ...Tool) Option {
	return func(o *ClaudeAgentOptions) {
		if len(tools) == 0 {
			return
		}

		server := createSDKToolServer(tools)

		if o.MCPServers == nil {
			o.MCPServers = make(map[string]MCPServerConfig, 1)
		}

		o.MCPServers["sdk"] = server

		for _, t := range tools {
			o.AllowedTools = append(o.AllowedTools, "mcp__sdk__"+t.Name())
		}
	}
}

// WithTransport injects a custom transport implementation.
// The transport must implement the Transport interface.
func WithTransport(transport config.Transport) Option {
	return func(o *ClaudeAgentOptions) {
		o.Transport = transport
	}
}
