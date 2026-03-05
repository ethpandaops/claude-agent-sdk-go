package protocol

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/hook"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/mcp"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/permission"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/userinput"
	"github.com/stretchr/testify/require"
)

// TestSession_NeedsInitialization_WithAgents tests that NeedsInitialization returns true
// when agents are configured, even without hooks, CanUseTool, or MCP servers.
func TestSession_NeedsInitialization_WithAgents(t *testing.T) {
	log := slog.Default()

	session := &Session{
		log: log,
		options: &config.Options{
			Agents: map[string]*config.AgentDefinition{
				"researcher": {
					Description: "A research agent",
					Prompt:      "You are a research assistant",
				},
			},
		},
		hookCallbacks: make(map[string]hook.Callback, 16),
		sdkMcpServers: make(map[string]mcp.ServerInstance, 4),
	}

	require.True(t, session.NeedsInitialization(),
		"Expected NeedsInitialization() to return true when agents are configured")
}

// TestSession_NeedsInitialization_Empty tests that NeedsInitialization returns false
// when no hooks, agents, CanUseTool, or MCP servers are configured.
func TestSession_NeedsInitialization_Empty(t *testing.T) {
	log := slog.Default()

	session := &Session{
		log:           log,
		options:       &config.Options{},
		hookCallbacks: make(map[string]hook.Callback, 16),
		sdkMcpServers: make(map[string]mcp.ServerInstance, 4),
	}

	require.False(t, session.NeedsInitialization(),
		"Expected NeedsInitialization() to return false with empty options")
}

// TestSession_InitializationResult_DataRace tests for data race between
// writing initializationResult and reading it via GetInitializationResult().
// Run with: go test -race -run TestSession_InitializationResult_DataRace.
func TestSession_InitializationResult_DataRace(t *testing.T) {
	log := slog.Default()

	// Create a session without a controller (we'll manipulate the field directly)
	session := &Session{
		log:           log,
		hookCallbacks: make(map[string]hook.Callback, 16),
		sdkMcpServers: make(map[string]mcp.ServerInstance, 4),
	}

	const iterations = 1000

	var wg sync.WaitGroup

	// Writer goroutine: simulates what Initialize() does (with mutex protection)

	wg.Go(func() {
		for i := range iterations {
			// This simulates what Initialize() does at line 141-143 (with mutex)
			session.initMu.Lock()
			session.initializationResult = map[string]any{
				"iteration": i,
				"data":      "test",
			}
			session.initMu.Unlock()
		}
	})

	// Reader goroutine: simulates concurrent GetInitializationResult() calls

	wg.Go(func() {
		for range iterations {
			// This calls the actual GetInitializationResult() which uses mutex
			result := session.GetInitializationResult()

			// Access the map to ensure the race detector catches any issues
			if result != nil {
				_ = len(result)
			}
		}
	})

	wg.Wait()
}

// TestSession_InitializationResult_ConcurrentReadWrite tests the race between
// a single write and multiple concurrent reads.
// Run with: go test -race -run TestSession_InitializationResult_ConcurrentReadWrite.
func TestSession_InitializationResult_ConcurrentReadWrite(t *testing.T) {
	log := slog.Default()

	session := &Session{
		log:           log,
		hookCallbacks: make(map[string]hook.Callback, 16),
		sdkMcpServers: make(map[string]mcp.ServerInstance, 4),
	}

	const (
		readers    = 10
		iterations = 1000
	)

	var wg sync.WaitGroup

	// Single writer (simulates Initialize with mutex protection)

	wg.Go(func() {
		for i := range iterations {
			session.initMu.Lock()
			session.initializationResult = map[string]any{
				"version": "1.0.0",
				"count":   i,
			}
			session.initMu.Unlock()
		}
	})

	// Multiple readers using GetInitializationResult()
	for range readers {
		wg.Go(func() {
			for range iterations {
				result := session.GetInitializationResult()
				if result != nil {
					// Access map contents - safe because we received a copy
					_ = result["version"]
					_ = result["count"]
				}
			}
		})
	}

	wg.Wait()
}

func TestSession_NeedsInitialization_WithOnUserInput(t *testing.T) {
	log := slog.Default()

	session := &Session{
		log: log,
		options: &config.Options{
			OnUserInput: func(_ context.Context, _ *userinput.Request) (*userinput.Response, error) {
				return &userinput.Response{}, nil
			},
		},
		hookCallbacks: make(map[string]hook.Callback, 16),
		sdkMcpServers: make(map[string]mcp.ServerInstance, 4),
	}

	require.True(t, session.NeedsInitialization())
}

func TestSession_HandleCanUseTool_OnUserInputAllow(t *testing.T) {
	log := slog.Default()

	session := NewSession(log, nil, &config.Options{
		OnUserInput: func(
			_ context.Context,
			req *userinput.Request,
		) (*userinput.Response, error) {
			require.Len(t, req.Questions, 1)
			require.Equal(t, "Which language should I use?", req.Questions[0].Question)

			return &userinput.Response{
				Answers: map[string]*userinput.Answer{
					"Which language should I use?": {Answers: []string{"Go"}},
				},
			}, nil
		},
	})

	resp, err := session.HandleCanUseTool(context.Background(), &ControlRequest{
		Request: map[string]any{
			"tool_name": "AskUserQuestion",
			"input": map[string]any{
				"questions": []any{
					map[string]any{
						"question":    "Which language should I use?",
						"header":      "Language",
						"multiSelect": false,
						"options": []any{
							map[string]any{"label": "Go", "description": "Simple and fast"},
							map[string]any{"label": "Rust", "description": "Safe and fast"},
						},
					},
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "allow", resp["behavior"])
	require.Contains(t, resp, "updatedInput")

	updatedInput, ok := resp["updatedInput"].(map[string]any)
	require.True(t, ok)

	answers, ok := updatedInput["answers"].(map[string]any)
	require.True(t, ok)

	answerValues, ok := answers["Which language should I use?"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"Go"}, answerValues)
}

func TestSession_HandleCanUseTool_OnUserInputErrorReturnsDeny(t *testing.T) {
	log := slog.Default()

	session := NewSession(log, nil, &config.Options{
		OnUserInput: func(
			_ context.Context,
			_ *userinput.Request,
		) (*userinput.Response, error) {
			return nil, context.DeadlineExceeded
		},
	})

	resp, err := session.HandleCanUseTool(context.Background(), &ControlRequest{
		Request: map[string]any{
			"tool_name": "AskUserQuestion",
			"input": map[string]any{
				"questions": []any{
					map[string]any{"question": "Which language should I use?"},
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "deny", resp["behavior"])
	require.Contains(t, resp["message"], "on_user_input callback error")
}

func TestSession_HandleCanUseTool_OnUserInputHasPriority(t *testing.T) {
	log := slog.Default()

	canUseToolCalled := false

	session := NewSession(log, nil, &config.Options{
		OnUserInput: func(
			_ context.Context,
			_ *userinput.Request,
		) (*userinput.Response, error) {
			return &userinput.Response{
				Answers: map[string]*userinput.Answer{
					"Which language should I use?": {Answers: []string{"Go"}},
				},
			}, nil
		},
		CanUseTool: func(
			_ context.Context,
			_ string,
			_ map[string]any,
			_ *permission.Context,
		) (permission.Result, error) {
			canUseToolCalled = true

			return &permission.ResultAllow{Behavior: "allow"}, nil
		},
	})

	resp, err := session.HandleCanUseTool(context.Background(), &ControlRequest{
		Request: map[string]any{
			"tool_name": "AskUserQuestion",
			"input": map[string]any{
				"questions": []any{
					map[string]any{"question": "Which language should I use?"},
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "allow", resp["behavior"])
	require.Contains(t, resp, "updatedInput")
	require.False(t, canUseToolCalled, "CanUseTool should not run for AskUserQuestion when OnUserInput is set")
}

func TestSession_HandleCanUseTool_NonAskUserQuestionUsesCanUseTool(t *testing.T) {
	log := slog.Default()

	canUseToolCalled := false

	session := NewSession(log, nil, &config.Options{
		OnUserInput: func(
			_ context.Context,
			_ *userinput.Request,
		) (*userinput.Response, error) {
			return &userinput.Response{}, nil
		},
		CanUseTool: func(
			_ context.Context,
			toolName string,
			_ map[string]any,
			_ *permission.Context,
		) (permission.Result, error) {
			canUseToolCalled = true

			require.Equal(t, "Bash", toolName)

			return &permission.ResultAllow{Behavior: "allow"}, nil
		},
	})

	resp, err := session.HandleCanUseTool(context.Background(), &ControlRequest{
		Request: map[string]any{
			"tool_name": "Bash",
			"input": map[string]any{
				"command": "echo hello",
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, "allow", resp["behavior"])
	require.Contains(t, resp, "updatedInput")
	require.True(t, canUseToolCalled)
}
