package cli

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"slices"
	"testing"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/errors"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/mcp"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/sandbox"
	"github.com/stretchr/testify/require"
)

const flagMCPConfig = "--mcp-config"

// TestDiscoverer_NotFound tests that an invalid CLI path returns CLINotFoundError.
func TestDiscoverer_NotFound(t *testing.T) {
	discoverer := NewDiscoverer(&Config{
		CliPath:          "/nonexistent/path/to/claude",
		SkipVersionCheck: true,
		Logger:           slog.Default(),
	})

	_, err := discoverer.Discover(context.Background())

	require.Error(t, err)
	require.IsType(t, &errors.CLINotFoundError{}, err)
}

// TestDiscoverer_ExplicitPath tests discovery with an explicit path.
func TestDiscoverer_ExplicitPath(t *testing.T) {
	// Create a temp file to act as the CLI
	tmpDir := t.TempDir()
	fakeCLI := tmpDir + "/claude"

	// Create the fake CLI file
	err := os.WriteFile(fakeCLI, []byte("#!/bin/sh\necho 2.1.0"), 0o755)
	require.NoError(t, err)

	discoverer := NewDiscoverer(&Config{
		CliPath:          fakeCLI,
		SkipVersionCheck: true,
		Logger:           slog.Default(),
	})

	path, err := discoverer.Discover(context.Background())

	require.NoError(t, err)
	require.Equal(t, fakeCLI, path)
}

// TestBuildArgs_Basic tests basic command building with minimal options.
func TestBuildArgs_Basic(t *testing.T) {
	options := &config.Options{}
	args := BuildArgs("hello", options, false)

	require.Contains(t, args, "--output-format")
	require.Contains(t, args, "stream-json")
	require.Contains(t, args, "--verbose")
	require.Contains(t, args, "--print")
	require.Contains(t, args, "--")
	require.Contains(t, args, "hello")
}

// TestBuildArgs_WithOptions tests command building with various options.
func TestBuildArgs_WithOptions(t *testing.T) {
	options := &config.Options{
		PermissionMode: "bypassPermissions",
		MaxTurns:       5,
		Model:          "claude-sonnet-4-6",
		SystemPrompt:   "You are helpful",
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, "--permission-mode")
	require.Contains(t, args, "bypassPermissions")
	require.Contains(t, args, "--max-turns")
	require.Contains(t, args, "5")
	require.Contains(t, args, "--model")
	require.Contains(t, args, "claude-sonnet-4-6")
	require.Contains(t, args, "--system-prompt")
	require.Contains(t, args, "You are helpful")
}

// TestBuildArgs_WithSystemPromptPreset tests system prompt preset configuration.
func TestBuildArgs_WithSystemPromptPreset(t *testing.T) {
	appendText := "\n\nAdditional context"
	options := &config.Options{
		SystemPromptPreset: &config.SystemPromptPreset{
			Type:   "preset",
			Preset: "claude_code",
			Append: &appendText,
		},
	}

	args := BuildArgs("test", options, false)

	// For presets with Append, use --append-system-prompt
	require.Contains(t, args, "--append-system-prompt")

	for i, arg := range args {
		if arg == "--append-system-prompt" && i+1 < len(args) {
			require.Contains(t, args[i+1], "Additional context")

			break
		}
	}
}

// TestBuildArgs_WithFallbackModel tests fallback model option.
func TestBuildArgs_WithFallbackModel(t *testing.T) {
	options := &config.Options{
		FallbackModel: "claude-sonnet-4-6",
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, "--fallback-model")
	require.Contains(t, args, "claude-sonnet-4-6")
}

// TestBuildArgs_WithAddDirs tests add-dir option.
func TestBuildArgs_WithAddDirs(t *testing.T) {
	options := &config.Options{
		AddDirs: []string{"/home/user/shared", "/opt/tools"},
	}

	args := BuildArgs("test", options, false)

	addDirCount := 0

	for _, arg := range args {
		if arg == "--add-dir" {
			addDirCount++
		}
	}

	require.Equal(t, 2, addDirCount)
	require.Contains(t, args, "/home/user/shared")
	require.Contains(t, args, "/opt/tools")
}

// TestBuildArgs_SessionContinuation tests session continuation options.
func TestBuildArgs_SessionContinuation(t *testing.T) {
	t.Run("continue conversation", func(t *testing.T) {
		options := &config.Options{
			ContinueConversation: true,
		}

		args := BuildArgs("test", options, false)

		require.Contains(t, args, "--continue")
	})

	t.Run("resume session", func(t *testing.T) {
		options := &config.Options{
			Resume: "session_abc123",
		}

		args := BuildArgs("test", options, false)

		require.Contains(t, args, "--resume")
		require.Contains(t, args, "session_abc123")
	})

	t.Run("fork session", func(t *testing.T) {
		options := &config.Options{
			Resume:      "session_xyz",
			ForkSession: true,
		}

		args := BuildArgs("test", options, false)

		require.Contains(t, args, "--resume")
		require.Contains(t, args, "--fork-session")
	})
}

// TestBuildArgs_WithSettingsFile tests settings file option.
func TestBuildArgs_WithSettingsFile(t *testing.T) {
	options := &config.Options{
		Settings: "/path/to/settings.json",
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, "--settings")
	require.Contains(t, args, "/path/to/settings.json")
}

// TestBuildArgs_WithSettingsJSON tests settings as JSON object.
func TestBuildArgs_WithSettingsJSON(t *testing.T) {
	settings := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Bash", "Read"},
		},
	}
	settingsJSON, err := json.Marshal(settings)
	require.NoError(t, err)

	options := &config.Options{
		Settings: string(settingsJSON),
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, "--settings")
}

// TestBuildArgs_WithSandboxOnly tests sandbox-only option (merged into --settings).
func TestBuildArgs_WithSandboxOnly(t *testing.T) {
	enabled := true
	options := &config.Options{
		SandboxSettings: &sandbox.Settings{
			Enabled: &enabled,
		},
	}

	args := BuildArgs("test", options, false)

	// Sandbox is now merged into --settings JSON, not a separate --sandbox flag
	require.Contains(t, args, "--settings")
	require.NotContains(t, args, "--sandbox")

	// Verify settings contains sandbox
	for i, arg := range args {
		if arg == "--settings" && i+1 < len(args) {
			require.Contains(t, args[i+1], "sandbox")

			break
		}
	}
}

// TestBuildArgs_WithSandboxAndSettingsJSON tests combined sandbox and settings.
func TestBuildArgs_WithSandboxAndSettingsJSON(t *testing.T) {
	enabled := true
	options := &config.Options{
		Settings: `{"test": "value"}`,
		SandboxSettings: &sandbox.Settings{
			Enabled: &enabled,
		},
	}

	args := BuildArgs("test", options, false)

	// Sandbox is merged into --settings JSON
	require.Contains(t, args, "--settings")
	require.NotContains(t, args, "--sandbox")

	// Verify settings contains both original settings and sandbox
	for i, arg := range args {
		if arg == "--settings" && i+1 < len(args) {
			require.Contains(t, args[i+1], "sandbox")
			require.Contains(t, args[i+1], "test")

			break
		}
	}
}

// TestBuildArgs_WithMCPServers tests MCP server configuration with mcpServers wrapper.
func TestBuildArgs_WithMCPServers(t *testing.T) {
	options := &config.Options{
		MCPServers: map[string]mcp.ServerConfig{
			"test-server": &mcp.StdioServerConfig{
				Type:    mcp.ServerTypeStdio,
				Command: "test-command",
				Args:    []string{"--arg1"},
			},
		},
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, flagMCPConfig)

	// Verify the mcpServers wrapper format
	for i, arg := range args {
		if arg == flagMCPConfig && i+1 < len(args) {
			mcpJSON := args[i+1]
			require.Contains(t, mcpJSON, `"mcpServers"`)
			require.Contains(t, mcpJSON, "test-server")

			break
		}
	}
}

// TestBuildArgs_WithMCPConfigFilePath tests MCP config as file path.
func TestBuildArgs_WithMCPConfigFilePath(t *testing.T) {
	options := &config.Options{
		MCPConfig: "/path/to/mcp-config.json",
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, flagMCPConfig)

	for i, arg := range args {
		if arg == flagMCPConfig && i+1 < len(args) {
			require.Equal(t, "/path/to/mcp-config.json", args[i+1])

			break
		}
	}
}

// TestBuildArgs_WithMCPConfigJSONString tests MCP config as raw JSON string.
func TestBuildArgs_WithMCPConfigJSONString(t *testing.T) {
	jsonConfig := `{"mcpServers": {"server": {"type": "stdio", "command": "test"}}}`
	options := &config.Options{
		MCPConfig: jsonConfig,
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, flagMCPConfig)

	for i, arg := range args {
		if arg == flagMCPConfig && i+1 < len(args) {
			require.Equal(t, jsonConfig, args[i+1])

			break
		}
	}
}

// TestBuildArgs_MCPConfigTakesPrecedence tests that MCPConfig takes precedence over MCPServers.
func TestBuildArgs_MCPConfigTakesPrecedence(t *testing.T) {
	options := &config.Options{
		MCPConfig: "/path/to/config.json",
		MCPServers: map[string]mcp.ServerConfig{
			"ignored": &mcp.StdioServerConfig{
				Type:    mcp.ServerTypeStdio,
				Command: "should-not-appear",
			},
		},
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, flagMCPConfig)

	// Verify only the file path appears, not the MCPServers config
	for i, arg := range args {
		if arg == flagMCPConfig && i+1 < len(args) {
			require.Equal(t, "/path/to/config.json", args[i+1])
			require.NotContains(t, args[i+1], "should-not-appear")

			break
		}
	}
}

// TestBuildEnvironment_EnvVarsPassedToSubprocess tests environment variable handling.
func TestBuildEnvironment_EnvVarsPassedToSubprocess(t *testing.T) {
	options := &config.Options{
		Env: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	env := BuildEnvironment(options)
	require.NotNil(t, env)

	require.True(t, slices.Contains(env, "CUSTOM_VAR=custom_value"),
		"Expected CUSTOM_VAR=custom_value in environment")
}

// TestBuildArgs_WithSystemPromptPresetAndAppend tests preset with appended text.
func TestBuildArgs_WithSystemPromptPresetAndAppend(t *testing.T) {
	appendText := "\n\nAdditional context for the assistant"
	options := &config.Options{
		SystemPromptPreset: &config.SystemPromptPreset{
			Type:   "preset",
			Preset: "claude_code",
			Append: &appendText,
		},
	}

	args := BuildArgs("test", options, false)

	// For presets with Append, use --append-system-prompt instead of --system-prompt
	require.Contains(t, args, "--append-system-prompt")
	require.NotContains(t, args, "--system-prompt")

	// Find the append-system-prompt value and verify it contains the appended text
	for i, arg := range args {
		if arg == "--append-system-prompt" && i+1 < len(args) {
			promptValue := args[i+1]
			require.Contains(t, promptValue, "Additional context for the assistant")

			break
		}
	}
}

// TestBuildArgs_WithExtraArgs tests arbitrary CLI flag passing.
func TestBuildArgs_WithExtraArgs(t *testing.T) {
	t.Run("boolean flag without value", func(t *testing.T) {
		options := &config.Options{
			ExtraArgs: map[string]*string{
				"debug-to-stderr": nil,
			},
		}

		args := BuildArgs("test", options, false)

		require.Contains(t, args, "--debug-to-stderr")
	})

	t.Run("flag with value", func(t *testing.T) {
		value := "custom-value"
		options := &config.Options{
			ExtraArgs: map[string]*string{
				"custom-flag": &value,
			},
		}

		args := BuildArgs("test", options, false)

		require.Contains(t, args, "--custom-flag")
		require.Contains(t, args, "custom-value")
	})

	t.Run("multiple extra args", func(t *testing.T) {
		valueA := "value-a"
		valueB := "value-b"
		options := &config.Options{
			ExtraArgs: map[string]*string{
				"flag-a":       &valueA,
				"flag-b":       &valueB,
				"boolean-flag": nil,
			},
		}

		args := BuildArgs("test", options, false)

		require.Contains(t, args, "--flag-a")
		require.Contains(t, args, "value-a")
		require.Contains(t, args, "--flag-b")
		require.Contains(t, args, "value-b")
		require.Contains(t, args, "--boolean-flag")
	})
}

// TestBuildArgs_WithSandboxNetworkConfig tests full sandbox network configuration.
func TestBuildArgs_WithSandboxNetworkConfig(t *testing.T) {
	enabled := true
	allowLocalBinding := true
	httpProxyPort := 8080
	socksProxyPort := 1080
	allowAllUnix := true

	options := &config.Options{
		SandboxSettings: &sandbox.Settings{
			Enabled: &enabled,
			Network: &sandbox.NetworkConfig{
				AllowUnixSockets:    []string{"/var/run/docker.sock"},
				AllowAllUnixSockets: &allowAllUnix,
				AllowLocalBinding:   &allowLocalBinding,
				HTTPProxyPort:       &httpProxyPort,
				SOCKSProxyPort:      &socksProxyPort,
			},
		},
	}

	args := BuildArgs("test", options, false)

	// Sandbox is merged into --settings JSON
	require.Contains(t, args, "--settings")
	require.NotContains(t, args, "--sandbox")
}

// TestBuildArgs_WithSandboxIgnoreViolations tests sandbox violation settings.
func TestBuildArgs_WithSandboxIgnoreViolations(t *testing.T) {
	enabled := true
	options := &config.Options{
		SandboxSettings: &sandbox.Settings{
			Enabled: &enabled,
			IgnoreViolations: &sandbox.IgnoreViolations{
				File:    []string{"/tmp/*", "/var/log/*"},
				Network: []string{"localhost:*"},
			},
		},
	}

	args := BuildArgs("test", options, false)

	// Sandbox is merged into --settings JSON
	require.Contains(t, args, "--settings")
	require.NotContains(t, args, "--sandbox")
}

// TestBuildArgs_WithOutputFormat tests JSON schema output format option.
func TestBuildArgs_WithOutputFormat(t *testing.T) {
	options := &config.Options{
		OutputFormat: map[string]any{
			"type": "json_schema",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "integer"},
				},
				"required": []string{"name"},
			},
		},
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, "--json-schema")

	// Verify the extracted schema is correct (inner schema, not wrapper)
	for i, arg := range args {
		if arg == "--json-schema" && i+1 < len(args) {
			schemaJSON := args[i+1]
			// Should contain the inner schema, not the wrapper
			require.Contains(t, schemaJSON, `"type":"object"`)
			require.NotContains(t, schemaJSON, `"type":"json_schema"`)

			break
		}
	}
}

// TestBuildArgs_WithOutputFormatRawSchema tests that raw JSON schemas
// (without the {"type": "json_schema", "schema": ...} wrapper) are
// auto-detected and produce the --json-schema flag.
func TestBuildArgs_WithOutputFormatRawSchema(t *testing.T) {
	tests := []struct {
		name         string
		outputFormat map[string]any
		wantFlag     bool
		wantContains string
	}{
		{
			name: "raw schema with properties",
			outputFormat: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []string{"name"},
			},
			wantFlag:     true,
			wantContains: `"name"`,
		},
		{
			name: "wrapped schema still works",
			outputFormat: map[string]any{
				"type": "json_schema",
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"age": map[string]any{"type": "integer"},
					},
				},
			},
			wantFlag:     true,
			wantContains: `"age"`,
		},
		{
			name:         "nil output format",
			outputFormat: nil,
			wantFlag:     false,
		},
		{
			name: "no properties and not json_schema type",
			outputFormat: map[string]any{
				"type": "text",
			},
			wantFlag: false,
		},
		{
			name:         "empty map",
			outputFormat: map[string]any{},
			wantFlag:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &config.Options{
				OutputFormat: tt.outputFormat,
			}

			args := BuildArgs("test", options, false)

			flagIdx := slices.Index(args, "--json-schema")
			if !tt.wantFlag {
				require.Equal(t, -1, flagIdx,
					"Expected --json-schema flag to be absent")

				return
			}

			require.NotEqual(t, -1, flagIdx,
				"Expected --json-schema flag to be present")
			require.Less(t, flagIdx+1, len(args),
				"Expected value after --json-schema flag")

			if tt.wantContains != "" {
				require.Contains(t, args[flagIdx+1], tt.wantContains)
			}
		})
	}
}

// TestBuildArgs_WithBetas tests beta feature flags.
func TestBuildArgs_WithBetas(t *testing.T) {
	options := &config.Options{
		Betas: []config.Beta{
			config.BetaContext1M,
		},
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, "--betas")
	require.Contains(t, args, string(config.BetaContext1M))
}

// TestBuildArgs_WithPermissionPromptToolName tests permission prompt tool option.
func TestBuildArgs_WithPermissionPromptToolName(t *testing.T) {
	options := &config.Options{
		PermissionPromptToolName: "custom_permission_tool",
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, "--permission-prompt-tool")
	require.Contains(t, args, "custom_permission_tool")
}

// TestBuildArgs_WithUser tests that user identifier is NOT passed as CLI flag.
// The User field is kept in options but not passed to CLI.
func TestBuildArgs_WithUser(t *testing.T) {
	options := &config.Options{
		User: "test-user-123",
	}

	args := BuildArgs("test", options, false)

	// --user is not a CLI flag
	require.NotContains(t, args, "--user")
}

// TestBuildArgs_WithMCPServersStdioConfig tests MCP server with full stdio configuration.
func TestBuildArgs_WithMCPServersStdioConfig(t *testing.T) {
	options := &config.Options{
		MCPServers: map[string]mcp.ServerConfig{
			"filesystem": &mcp.StdioServerConfig{
				Type:    mcp.ServerTypeStdio,
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/home/user"},
				Env: map[string]string{
					"NODE_ENV": "production",
				},
			},
		},
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, flagMCPConfig)

	// Find and validate the JSON config
	for i, arg := range args {
		if arg == flagMCPConfig && i+1 < len(args) {
			mcpJSON := args[i+1]
			// Verify it contains expected keys
			require.Contains(t, mcpJSON, "filesystem")
			require.Contains(t, mcpJSON, "npx")
			require.Contains(t, mcpJSON, "@modelcontextprotocol/server-filesystem")

			break
		}
	}
}

// TestBuildArgs_WithMCPServersSSEConfig tests MCP server with SSE configuration.
func TestBuildArgs_WithMCPServersSSEConfig(t *testing.T) {
	options := &config.Options{
		MCPServers: map[string]mcp.ServerConfig{
			"remote-server": &mcp.SSEServerConfig{
				Type: mcp.ServerTypeSSE,
				URL:  "https://api.example.com/mcp",
			},
		},
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, flagMCPConfig)

	// Find and validate the JSON config
	for i, arg := range args {
		if arg == flagMCPConfig && i+1 < len(args) {
			mcpJSON := args[i+1]
			require.Contains(t, mcpJSON, "remote-server")
			require.Contains(t, mcpJSON, "https://api.example.com/mcp")
			require.Contains(t, mcpJSON, "sse")

			break
		}
	}
}

// TestBuildArgs_WithMultipleMCPServers tests multiple MCP servers configured together.
func TestBuildArgs_WithMultipleMCPServers(t *testing.T) {
	options := &config.Options{
		MCPServers: map[string]mcp.ServerConfig{
			"local-server": &mcp.StdioServerConfig{
				Type:    mcp.ServerTypeStdio,
				Command: "node",
				Args:    []string{"server.js"},
			},
			"remote-server": &mcp.SSEServerConfig{
				Type: mcp.ServerTypeSSE,
				URL:  "https://remote.example.com/mcp",
			},
		},
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, flagMCPConfig)

	// Find and validate the JSON config contains both servers
	for i, arg := range args {
		if arg == flagMCPConfig && i+1 < len(args) {
			mcpJSON := args[i+1]
			require.Contains(t, mcpJSON, "local-server")
			require.Contains(t, mcpJSON, "remote-server")
			require.Contains(t, mcpJSON, "node")
			require.Contains(t, mcpJSON, "https://remote.example.com/mcp")

			break
		}
	}
}

// TestBuildArgs_WithMCPServersEnvVars tests MCP server with environment variables.
func TestBuildArgs_WithMCPServersEnvVars(t *testing.T) {
	options := &config.Options{
		MCPServers: map[string]mcp.ServerConfig{
			"api-server": &mcp.StdioServerConfig{
				Type:    mcp.ServerTypeStdio,
				Command: "node",
				Args:    []string{"server.js"},
				Env: map[string]string{
					"API_KEY":   "secret-key",
					"DEBUG":     "true",
					"LOG_LEVEL": "verbose",
				},
			},
		},
	}

	args := BuildArgs("test", options, false)

	require.Contains(t, args, flagMCPConfig)

	// Find and validate the JSON config contains env vars
	for i, arg := range args {
		if arg == flagMCPConfig && i+1 < len(args) {
			mcpJSON := args[i+1]
			require.Contains(t, mcpJSON, "API_KEY")
			require.Contains(t, mcpJSON, "secret-key")
			require.Contains(t, mcpJSON, "DEBUG")

			break
		}
	}
}

// TestBuildArgs_WithoutTools tests that no --tools flag appears when BuiltinTools is nil.
func TestBuildArgs_WithoutTools(t *testing.T) {
	options := &config.Options{
		// Tools is nil (default) - should not include --tools flag
	}

	args := BuildArgs("test", options, false)

	// Verify --tools flag is NOT present
	require.NotContains(t, args, "--tools",
		"Expected --tools flag to be absent when Tools is nil")
}

// TestBuildArgs_WithToolsList tests that Tools as ToolsList maps to --tools flag.
func TestBuildArgs_WithToolsList(t *testing.T) {
	options := &config.Options{
		Tools: config.ToolsList{"Read", "Write", "Glob"},
	}

	args := BuildArgs("test", options, false)

	// Verify --tools flag is present with correct value
	toolsIdx := slices.Index(args, "--tools")
	require.NotEqual(t, -1, toolsIdx, "Expected --tools flag to be present")
	require.Less(t, toolsIdx+1, len(args), "Expected value after --tools flag")
	require.Equal(t, "Read,Write,Glob", args[toolsIdx+1])
}

// TestBuildArgs_WithEmptyToolsList tests that empty Tools ToolsList maps to --tools "".
func TestBuildArgs_WithEmptyToolsList(t *testing.T) {
	options := &config.Options{
		Tools: config.ToolsList{},
	}

	args := BuildArgs("test", options, false)

	// Verify --tools flag is present with empty value
	toolsIdx := slices.Index(args, "--tools")
	require.NotEqual(t, -1, toolsIdx, "Expected --tools flag to be present for empty tools list")
	require.Less(t, toolsIdx+1, len(args), "Expected value after --tools flag")
	require.Equal(t, "", args[toolsIdx+1])
}

// TestBuildArgs_WithToolsPreset tests that Tools as *ToolsPreset maps to --tools default.
func TestBuildArgs_WithToolsPreset(t *testing.T) {
	options := &config.Options{
		Tools: &config.ToolsPreset{
			Type:   "preset",
			Preset: "claude_code",
		},
	}

	args := BuildArgs("test", options, false)

	// Verify --tools flag is present with 'default' value
	toolsIdx := slices.Index(args, "--tools")
	require.NotEqual(t, -1, toolsIdx, "Expected --tools flag to be present")
	require.Less(t, toolsIdx+1, len(args), "Expected value after --tools flag")
	require.Equal(t, "default", args[toolsIdx+1])
}

// TestBuildArgs_WithAllowedToolsOnly tests AllowedTools maps to --allowed-tools.
func TestBuildArgs_WithAllowedToolsOnly(t *testing.T) {
	options := &config.Options{
		AllowedTools: []string{"Bash(git:*)", "Read"},
	}

	args := BuildArgs("test", options, false)

	// Verify --allowed-tools flag is present
	allowedIdx := slices.Index(args, "--allowed-tools")
	require.NotEqual(t, -1, allowedIdx, "Expected --allowed-tools flag to be present")
	require.Less(t, allowedIdx+1, len(args), "Expected value after --allowed-tools flag")
	require.Equal(t, "Bash(git:*),Read", args[allowedIdx+1])

	// Verify --tools flag is NOT present
	require.NotContains(t, args, "--tools")
}

// TestBuildArgs_WithDisallowedToolsOnly tests DisallowedTools maps to --disallowed-tools.
func TestBuildArgs_WithDisallowedToolsOnly(t *testing.T) {
	options := &config.Options{
		DisallowedTools: []string{"Bash", "Write"},
	}

	args := BuildArgs("test", options, false)

	// Verify --disallowed-tools flag is present
	disallowedIdx := slices.Index(args, "--disallowed-tools")
	require.NotEqual(t, -1, disallowedIdx, "Expected --disallowed-tools flag to be present")
	require.Less(t, disallowedIdx+1, len(args), "Expected value after --disallowed-tools flag")
	require.Equal(t, "Bash,Write", args[disallowedIdx+1])
}

// TestBuildArgs_WithToolsAndAllowedTools tests Tools and AllowedTools can be combined.
func TestBuildArgs_WithToolsAndAllowedTools(t *testing.T) {
	options := &config.Options{
		Tools:        config.ToolsList{"Read", "Glob", "Grep"},
		AllowedTools: []string{"Bash(git:*)"},
	}

	args := BuildArgs("test", options, false)

	// Verify both flags are present
	toolsIdx := slices.Index(args, "--tools")
	require.NotEqual(t, -1, toolsIdx, "Expected --tools flag to be present")
	require.Equal(t, "Read,Glob,Grep", args[toolsIdx+1])

	allowedIdx := slices.Index(args, "--allowed-tools")
	require.NotEqual(t, -1, allowedIdx, "Expected --allowed-tools flag to be present")
	require.Equal(t, "Bash(git:*)", args[allowedIdx+1])
}

// TestBuildArgs_WithToolsAndDisallowedTools tests Tools and DisallowedTools can be combined.
func TestBuildArgs_WithToolsAndDisallowedTools(t *testing.T) {
	options := &config.Options{
		Tools:           config.ToolsList{"Read", "Glob", "Grep", "Write"},
		DisallowedTools: []string{"Write"},
	}

	args := BuildArgs("test", options, false)

	// Verify both flags are present
	toolsIdx := slices.Index(args, "--tools")
	require.NotEqual(t, -1, toolsIdx, "Expected --tools flag to be present")
	require.Equal(t, "Read,Glob,Grep,Write", args[toolsIdx+1])

	disallowedIdx := slices.Index(args, "--disallowed-tools")
	require.NotEqual(t, -1, disallowedIdx, "Expected --disallowed-tools flag to be present")
	require.Equal(t, "Write", args[disallowedIdx+1])
}

// TestBuildArgs_WithAllToolOptions tests all three tool options combined.
func TestBuildArgs_WithAllToolOptions(t *testing.T) {
	options := &config.Options{
		Tools:           config.ToolsList{"Read", "Glob", "Grep", "Write", "Bash"},
		AllowedTools:    []string{"Bash(npm:*)"},
		DisallowedTools: []string{"Bash(rm:*)"},
	}

	args := BuildArgs("test", options, false)

	// Verify all three flags are present
	toolsIdx := slices.Index(args, "--tools")
	require.NotEqual(t, -1, toolsIdx)
	require.Equal(t, "Read,Glob,Grep,Write,Bash", args[toolsIdx+1])

	allowedIdx := slices.Index(args, "--allowed-tools")
	require.NotEqual(t, -1, allowedIdx)
	require.Equal(t, "Bash(npm:*)", args[allowedIdx+1])

	disallowedIdx := slices.Index(args, "--disallowed-tools")
	require.NotEqual(t, -1, disallowedIdx)
	require.Equal(t, "Bash(rm:*)", args[disallowedIdx+1])
}

// TestBuildArgs_WithSettingsFileAndNoSandbox tests settings file is passed without sandbox.
func TestBuildArgs_WithSettingsFileAndNoSandbox(t *testing.T) {
	options := &config.Options{
		Settings: "/custom/path/to/settings.json",
		// No sandbox settings
	}

	args := BuildArgs("test", options, false)

	// Verify settings file is passed
	require.Contains(t, args, "--settings")
	require.Contains(t, args, "/custom/path/to/settings.json")

	// Verify sandbox is not enabled
	require.NotContains(t, args, "--sandbox")
}

// TestBuildArgs_StreamingMode tests streaming mode command building.
func TestBuildArgs_StreamingMode(t *testing.T) {
	options := &config.Options{
		PermissionMode: "acceptEdits",
	}

	// Test streaming mode
	args := BuildArgs("ignored prompt", options, true)

	require.Contains(t, args, "--input-format")
	require.Contains(t, args, "stream-json")
	// In streaming mode, --print and prompt should NOT be present
	require.NotContains(t, args, "--print")
	require.NotContains(t, args, "ignored prompt")
}

// TestBuildArgs_NonStreamingMode tests non-streaming mode command building.
func TestBuildArgs_NonStreamingMode(t *testing.T) {
	options := &config.Options{
		PermissionMode: "acceptEdits",
	}

	// Test non-streaming mode (default)
	args := BuildArgs("test prompt", options, false)

	require.Contains(t, args, "--print")
	require.Contains(t, args, "test prompt")
	// In non-streaming mode, --input-format stream-json should NOT be present
	require.NotContains(t, args, "--input-format")
}

// TestBuildArgs_AgentsNotInCLIArgs tests that agents are NOT passed as CLI flags.
// Agent definitions are sent via the initialize control request instead.
func TestBuildArgs_AgentsNotInCLIArgs(t *testing.T) {
	options := &config.Options{
		Agents: map[string]*config.AgentDefinition{
			"researcher": {
				Description: "A research agent",
				Prompt:      "You are a research assistant",
				Tools:       []string{"Read", "Glob", "Grep"},
			},
		},
	}

	args := BuildArgs("test", options, false)

	require.NotContains(t, args, "--agents",
		"Expected --agents flag to be absent; agents are sent via initialize request")
}

// TestBuildArgs_WithThinkingConfigAdaptive tests adaptive thinking config.
func TestBuildArgs_WithThinkingConfigAdaptive(t *testing.T) {
	options := &config.Options{
		Thinking: config.ThinkingConfigAdaptive{},
	}

	args := BuildArgs("test", options, false)

	// Adaptive defaults to 32000
	thinkingIdx := slices.Index(args, "--max-thinking-tokens")
	require.NotEqual(t, -1, thinkingIdx, "Expected --max-thinking-tokens flag")
	require.Less(t, thinkingIdx+1, len(args))
	require.Equal(t, "32000", args[thinkingIdx+1])
}

// TestBuildArgs_WithThinkingConfigEnabled tests enabled thinking config with budget.
func TestBuildArgs_WithThinkingConfigEnabled(t *testing.T) {
	options := &config.Options{
		Thinking: config.ThinkingConfigEnabled{BudgetTokens: 50000},
	}

	args := BuildArgs("test", options, false)

	thinkingIdx := slices.Index(args, "--max-thinking-tokens")
	require.NotEqual(t, -1, thinkingIdx, "Expected --max-thinking-tokens flag")
	require.Less(t, thinkingIdx+1, len(args))
	require.Equal(t, "50000", args[thinkingIdx+1])
}

// TestBuildArgs_WithThinkingConfigDisabled tests disabled thinking config.
func TestBuildArgs_WithThinkingConfigDisabled(t *testing.T) {
	options := &config.Options{
		Thinking: config.ThinkingConfigDisabled{},
	}

	args := BuildArgs("test", options, false)

	thinkingIdx := slices.Index(args, "--max-thinking-tokens")
	require.NotEqual(t, -1, thinkingIdx, "Expected --max-thinking-tokens flag")
	require.Less(t, thinkingIdx+1, len(args))
	require.Equal(t, "0", args[thinkingIdx+1])
}

// TestBuildArgs_WithEffort tests the effort flag.
func TestBuildArgs_WithEffort(t *testing.T) {
	tests := []struct {
		name     string
		effort   config.Effort
		expected string
	}{
		{name: "low", effort: config.EffortLow, expected: "low"},
		{name: "medium", effort: config.EffortMedium, expected: "medium"},
		{name: "high", effort: config.EffortHigh, expected: "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effort := tt.effort
			options := &config.Options{
				Effort: &effort,
			}

			args := BuildArgs("test", options, false)

			effortIdx := slices.Index(args, "--effort")
			require.NotEqual(t, -1, effortIdx, "Expected --effort flag")
			require.Less(t, effortIdx+1, len(args))
			require.Equal(t, tt.expected, args[effortIdx+1])
		})
	}
}

// TestCompareVersions tests semantic version comparison.
func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		// Equal versions
		{name: "equal versions", a: "1.0.0", b: "1.0.0", expected: 0},
		{name: "equal versions 2", a: "2.5.10", b: "2.5.10", expected: 0},

		// A < B (should return -1)
		{name: "major version less", a: "1.0.0", b: "2.0.0", expected: -1},
		{name: "minor version less", a: "1.0.0", b: "1.1.0", expected: -1},
		{name: "patch version less", a: "1.0.0", b: "1.0.1", expected: -1},
		{name: "complex less", a: "1.9.9", b: "2.0.0", expected: -1},
		{name: "minor rollover", a: "1.99.0", b: "2.0.0", expected: -1},

		// A > B (should return 1)
		{name: "major version greater", a: "2.0.0", b: "1.0.0", expected: 1},
		{name: "minor version greater", a: "1.1.0", b: "1.0.0", expected: 1},
		{name: "patch version greater", a: "1.0.1", b: "1.0.0", expected: 1},
		{name: "complex greater", a: "2.0.0", b: "1.9.9", expected: 1},

		// Minimum version check (2.0.0 is minimum)
		{name: "below minimum", a: "1.9.9", b: "2.0.0", expected: -1},
		{name: "at minimum", a: "2.0.0", b: "2.0.0", expected: 0},
		{name: "above minimum", a: "2.1.0", b: "2.0.0", expected: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.a, tt.b)
			require.Equal(t, tt.expected, result, "compareVersions(%q, %q)", tt.a, tt.b)
		})
	}
}
