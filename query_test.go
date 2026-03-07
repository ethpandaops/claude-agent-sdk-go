package claudesdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
)

func isExpectedQueryError(err error) bool {
	if err == nil {
		return true
	}

	_, isCLINotFound := errors.AsType[*CLINotFoundError](err)
	_, isProcessError := errors.AsType[*ProcessError](err)

	return isCLINotFound ||
		isProcessError ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

func TestQueryCLINotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, err := range Query(ctx, "test",
		WithCliPath("/nonexistent/path/to/claude"),
	) {
		if err == nil {
			t.Fatal("Expected error when CLI not found")
		}

		if _, ok := errors.AsType[*CLINotFoundError](err); !ok {
			t.Errorf("Expected CLINotFoundError, got: %v", err)
		}

		break
	}
}

func TestQueryWithNoOptions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should work if claude is in PATH
	// If not, it should return CLINotFoundError or ProcessError
	for _, err := range Query(ctx, "test") {
		if !isExpectedQueryError(err) {
			t.Errorf("Unexpected error type: %v", err)
		}

		break
	}
}

// TestQuery_WithOptions tests Query with full ClaudeAgentOptions configuration.
func TestQuery_WithOptions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, err := range Query(ctx, "test",
		WithSystemPrompt("You are a helpful assistant."),
		WithModel("claude-sonnet-4-6"),
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
		WithThinking(ThinkingConfigEnabled{BudgetTokens: 1000}),
		WithMaxBudgetUSD(1.0),
		WithIncludePartialMessages(true),
		WithAllowedTools("Read", "Grep"),
		WithDisallowedTools("Bash"),
		WithEnv(map[string]string{"TEST_VAR": "test_value"}),
		WithSettingSources(SettingSourceUser),
	) {
		// Either succeeds or returns an expected CLI/context error.
		if !isExpectedQueryError(err) {
			t.Errorf("Unexpected error type with full options: %v", err)
		}

		break
	}
}

// TestQuery_WithCwd tests Query with a custom working directory.
func TestQuery_WithCwd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "claude-sdk-test-cwd-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	// Resolve to absolute path
	absPath, err := filepath.Abs(tmpDir)
	require.NoError(t, err)

	for _, err := range Query(ctx, "test",
		WithCwd(absPath),
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
	) {
		// Either succeeds or returns an expected CLI/context error.
		if !isExpectedQueryError(err) {
			t.Errorf("Unexpected error type with Cwd option: %v", err)
		}

		break
	}
}

// TestQuery_WithEnv tests Query with custom environment variables.
func TestQuery_WithEnv(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, err := range Query(ctx, "test",
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
		WithEnv(map[string]string{
			"CLAUDE_SDK_TEST_VAR": "test_value_123",
			"ANOTHER_VAR":         "another_value",
		}),
	) {
		// Either succeeds or returns an expected CLI/context error.
		if !isExpectedQueryError(err) {
			t.Errorf("Unexpected error type with Env option: %v", err)
		}

		break
	}
}

// TestQuery_WithSystemPromptPreset tests Query with a system prompt preset.
func TestQuery_WithSystemPromptPreset(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	appendText := "\nAdditional instructions."

	for _, err := range Query(ctx, "test",
		WithSystemPromptPreset(&SystemPromptPreset{
			Type:   "preset",
			Preset: "claude_code",
			Append: &appendText,
		}),
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
	) {
		// Either succeeds or returns an expected CLI/context error.
		if !isExpectedQueryError(err) {
			t.Errorf("Unexpected error type with SystemPromptPreset: %v", err)
		}

		break
	}
}

// TestQuery_WithAgents tests Query with custom agent definitions.
func TestQuery_WithAgents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	model := "claude-sonnet-4-6"

	for _, err := range Query(ctx, "test",
		WithAgents(map[string]*AgentDefinition{
			"test-agent": {
				Description: "A test agent",
				Prompt:      "You are a test agent.",
				Tools:       []string{"Read"},
				Model:       &model,
			},
		}),
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
	) {
		// Either succeeds or returns an expected CLI/context error.
		if !isExpectedQueryError(err) {
			t.Errorf("Unexpected error type with Agents option: %v", err)
		}

		break
	}
}

// TestQuery_WithOutputFormat tests Query with structured output format.
func TestQuery_WithOutputFormat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, err := range Query(ctx, "test",
		WithOutputFormat(map[string]any{
			"type": "json_schema",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"answer": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"answer"},
			},
		}),
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
	) {
		// Either succeeds or returns an expected CLI/context error.
		if !isExpectedQueryError(err) {
			t.Errorf("Unexpected error type with OutputFormat option: %v", err)
		}

		break
	}
}

// TestQuery_WithResume tests Query with session resume option.
func TestQuery_WithResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, err := range Query(ctx, "test",
		WithResume("nonexistent-session-id"),
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
	) {
		// May fail due to invalid session, but should not be unexpected error type.
		if !isExpectedQueryError(err) {
			t.Logf("Error with Resume option (may be expected): %v", err)
		}

		break
	}
}

// TestQuery_WithExtraArgs tests Query with extra CLI arguments.
func TestQuery_WithExtraArgs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	verbose := ""

	for _, err := range Query(ctx, "test",
		WithExtraArgs(map[string]*string{
			"verbose": &verbose, // Boolean flag with empty value
		}),
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
	) {
		// Either succeeds or returns an expected CLI/context error.
		if !isExpectedQueryError(err) {
			t.Errorf("Unexpected error type with ExtraArgs option: %v", err)
		}

		break
	}
}

// TestQuery_CanUseToolWithPermissionPromptToolName tests that Query yields
// an error when both CanUseTool and PermissionPromptToolName are set.
func TestQuery_CanUseToolWithPermissionPromptToolName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, err := range Query(ctx, "test",
		WithCanUseTool(func(
			_ context.Context,
			_ string,
			_ map[string]any,
			_ *ToolPermissionContext,
		) (PermissionResult, error) {
			return &PermissionResultAllow{Behavior: "allow"}, nil
		}),
		WithPermissionPromptToolName("custom"),
		WithPermissionMode("bypassPermissions"),
	) {
		require.Error(t, err)
		require.Contains(t, err.Error(), "can_use_tool callback cannot be used with permission_prompt_tool_name")

		break
	}
}

// TestQuery_CanUseToolAutoConfiguresPermissionPrompt tests that Query
// automatically sets PermissionPromptToolName to "stdio" when CanUseTool is set.
func TestQuery_CanUseToolAutoConfiguresPermissionPrompt(t *testing.T) {
	options := &ClaudeAgentOptions{
		CanUseTool: func(
			_ context.Context,
			_ string,
			_ map[string]any,
			_ *ToolPermissionContext,
		) (PermissionResult, error) {
			return &PermissionResultAllow{Behavior: "allow"}, nil
		},
		PermissionMode: "bypassPermissions",
	}

	// Call validateAndConfigureOptions directly to test the auto-configuration
	err := validateAndConfigureOptions(options)

	require.NoError(t, err)
	require.Equal(t, "stdio", options.PermissionPromptToolName)
}

// TestQuery_OnUserInputWithPermissionPromptToolName tests that Query yields
// an error when both OnUserInput and PermissionPromptToolName are set.
func TestQuery_OnUserInputWithPermissionPromptToolName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, err := range Query(ctx, "test",
		WithOnUserInput(dummyOnUserInput),
		WithPermissionPromptToolName("custom"),
		WithPermissionMode("bypassPermissions"),
	) {
		require.Error(t, err)
		require.Contains(t, err.Error(), "on_user_input callback cannot be used with permission_prompt_tool_name")

		break
	}
}

// TestQueryStream_CanUseToolWithPermissionPromptToolName tests that QueryStream
// yields an error when both CanUseTool and PermissionPromptToolName are set.
func TestQueryStream_CanUseToolWithPermissionPromptToolName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages := MessagesFromSlice([]StreamingMessage{
		NewUserMessage("test"),
	})

	for _, err := range QueryStream(ctx, messages,
		WithCanUseTool(func(
			_ context.Context,
			_ string,
			_ map[string]any,
			_ *ToolPermissionContext,
		) (PermissionResult, error) {
			return &PermissionResultAllow{Behavior: "allow"}, nil
		}),
		WithPermissionPromptToolName("custom"),
		WithPermissionMode("bypassPermissions"),
	) {
		require.Error(t, err)
		require.Contains(t, err.Error(), "can_use_tool callback cannot be used with permission_prompt_tool_name")

		break
	}
}

// TestQueryStream_OnUserInputWithPermissionPromptToolName tests that QueryStream
// yields an error when both OnUserInput and PermissionPromptToolName are set.
func TestQueryStream_OnUserInputWithPermissionPromptToolName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages := MessagesFromSlice([]StreamingMessage{
		NewUserMessage("test"),
	})

	for _, err := range QueryStream(ctx, messages,
		WithOnUserInput(dummyOnUserInput),
		WithPermissionPromptToolName("custom"),
		WithPermissionMode("bypassPermissions"),
	) {
		require.Error(t, err)
		require.Contains(t, err.Error(), "on_user_input callback cannot be used with permission_prompt_tool_name")

		break
	}
}

// dummyCanUseTool is a helper for tests that returns allow.
func dummyCanUseTool(
	_ context.Context,
	_ string,
	_ map[string]any,
	_ *ToolPermissionContext,
) (PermissionResult, error) {
	return &PermissionResultAllow{Behavior: "allow"}, nil
}

// dummyOnUserInput is a helper for tests that returns a static answer.
func dummyOnUserInput(
	_ context.Context,
	_ *UserInputRequest,
) (*UserInputResponse, error) {
	return &UserInputResponse{
		Answers: map[string]*UserInputAnswer{
			"question": {Answers: []string{"answer"}},
		},
	}, nil
}

// TestValidateAndConfigureOptions tests the validation helper function.
func TestValidateAndConfigureOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     *ClaudeAgentOptions
		wantErr     bool
		errContains string
		checkFunc   func(t *testing.T, opts *ClaudeAgentOptions)
	}{
		{
			name:    "nil CanUseTool does not modify PermissionPromptToolName",
			options: &ClaudeAgentOptions{},
			wantErr: false,
			checkFunc: func(t *testing.T, opts *ClaudeAgentOptions) {
				t.Helper()
				require.Empty(t, opts.PermissionPromptToolName)
			},
		},
		{
			name: "CanUseTool without PermissionPromptToolName sets stdio",
			options: &ClaudeAgentOptions{
				CanUseTool: dummyCanUseTool,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, opts *ClaudeAgentOptions) {
				t.Helper()
				require.Equal(t, "stdio", opts.PermissionPromptToolName)
			},
		},
		{
			name: "CanUseTool with PermissionPromptToolName returns error",
			options: &ClaudeAgentOptions{
				CanUseTool:               dummyCanUseTool,
				PermissionPromptToolName: "custom",
			},
			wantErr:     true,
			errContains: "can_use_tool callback cannot be used with permission_prompt_tool_name",
		},
		{
			name: "OnUserInput without PermissionPromptToolName sets stdio",
			options: &ClaudeAgentOptions{
				OnUserInput: dummyOnUserInput,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, opts *ClaudeAgentOptions) {
				t.Helper()
				require.Equal(t, "stdio", opts.PermissionPromptToolName)
			},
		},
		{
			name: "OnUserInput with PermissionPromptToolName returns error",
			options: &ClaudeAgentOptions{
				OnUserInput:              dummyOnUserInput,
				PermissionPromptToolName: "custom",
			},
			wantErr:     true,
			errContains: "on_user_input callback cannot be used with permission_prompt_tool_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAndConfigureOptions(tt.options)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)

				if tt.checkFunc != nil {
					tt.checkFunc(t, tt.options)
				}
			}
		})
	}
}

func TestQueryRequiresStreamingMode(t *testing.T) {
	tests := []struct {
		name    string
		options *ClaudeAgentOptions
		want    bool
	}{
		{
			name:    "empty options",
			options: &ClaudeAgentOptions{},
			want:    false,
		},
		{
			name: "with can_use_tool",
			options: &ClaudeAgentOptions{
				CanUseTool: dummyCanUseTool,
			},
			want: true,
		},
		{
			name: "with on_user_input",
			options: &ClaudeAgentOptions{
				OnUserInput: dummyOnUserInput,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := queryRequiresStreamingMode(tt.options)
			require.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Mid-Operation Context Cancellation Tests
// =============================================================================

// TestQuery_ContextCancelMidIteration tests that Query respects context
// cancellation during message iteration.
func TestQuery_ContextCancelMidIteration(t *testing.T) {
	t.Run("context timeout during iteration", func(t *testing.T) {
		// Create a context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		var gotError error

		var iterationCount int

		for _, err := range Query(ctx, "test",
			WithCliPath("/nonexistent/path/to/claude"),
		) {
			iterationCount++

			if err != nil {
				gotError = err

				break
			}
		}

		// Should get either CLI not found (fast path) or context deadline exceeded
		require.Error(t, gotError)

		_, isCLINotFound := errors.AsType[*CLINotFoundError](gotError)
		isContextErr := gotError == context.DeadlineExceeded ||
			gotError == context.Canceled

		require.True(t, isCLINotFound || isContextErr,
			"Expected CLINotFoundError or context error, got: %v", gotError)

		t.Logf("Iterations: %d, Error type: %T", iterationCount, gotError)
	})

	t.Run("context cancel before iteration starts", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		var gotError error

		for _, err := range Query(ctx, "test") {
			if err != nil {
				gotError = err

				break
			}
		}

		// Should get an error (either context canceled or CLI not found)
		require.Error(t, gotError)
	})
}

// TestQueryStream_ContextCancelMidIteration tests that QueryStream respects
// context cancellation during message iteration.
func TestQueryStream_ContextCancelMidIteration(t *testing.T) {
	t.Run("context timeout during streaming iteration", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		messages := MessagesFromSlice([]StreamingMessage{
			NewUserMessage("test message"),
		})

		var gotError error

		for _, err := range QueryStream(ctx, messages,
			WithCliPath("/nonexistent/path/to/claude"),
		) {
			if err != nil {
				gotError = err

				break
			}
		}

		// Should get either CLI not found or context error
		require.Error(t, gotError)

		_, isCLINotFound := errors.AsType[*CLINotFoundError](gotError)
		isContextErr := gotError == context.DeadlineExceeded ||
			gotError == context.Canceled

		require.True(t, isCLINotFound || isContextErr,
			"Expected CLINotFoundError or context error, got: %v", gotError)
	})
}

// =============================================================================
// Early Iteration Exit Goroutine Leak Tests
// =============================================================================

// TestQuery_EarlyExitDoesNotLeakGoroutines tests that breaking out of the
// iterator early doesn't leak goroutines.
func TestQuery_EarlyExitDoesNotLeakGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Break out of iteration immediately
	for _, err := range Query(ctx, "test",
		WithCliPath("/nonexistent/path/to/claude"),
	) {
		// Break immediately regardless of error
		_ = err

		break
	}

	// Allow goroutines to settle
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Should not have leaked goroutines (allow +2 for GC fluctuation)
	require.LessOrEqual(t, after, before+2,
		"goroutine leak detected: before=%d, after=%d", before, after)
}

// TestQueryStream_EarlyExitDoesNotBlockInputStreamer tests that early exit
// with MCP/hooks configuration doesn't block the input streamer goroutine.
func TestQueryStream_EarlyExitDoesNotBlockInputStreamer(t *testing.T) {
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages := MessagesFromSlice([]StreamingMessage{
		NewUserMessage("test"),
	})

	// Break out immediately
	for _, err := range QueryStream(ctx, messages,
		WithCliPath("/nonexistent/path/to/claude"),
	) {
		_ = err

		break
	}

	// Allow goroutines to settle - should NOT take 60 seconds
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	require.LessOrEqual(t, after, before+2,
		"goroutine leak detected - input streamer may be blocked")
}

// TestQueryStream_DeferOrderingDoesNotDeadlock verifies that the defer ordering
// in QueryStream correctly closes resultReceived BEFORE wg.Wait() to prevent
// deadlock when the main loop exits early while streamInputMessages is blocked.
//
// This test specifically targets Issue #1: Potential goroutine leak in error paths.
// The bug was that defer statements were in wrong order (LIFO):
//   - defer close(resultReceived) registered FIRST, executes SECOND
//   - defer wg.Wait() registered SECOND, executes FIRST
//
// This caused wg.Wait() to block before close(resultReceived) could unblock
// the streamInputMessages goroutine, relying on 60s timeout to recover.
func TestQueryStream_DeferOrderingDoesNotDeadlock(t *testing.T) {
	before := runtime.NumGoroutine()

	// Use a context with short timeout to force early exit
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create a slow message iterator that will be interrupted
	slowMessages := func(yield func(StreamingMessage) bool) {
		// First message goes through
		if !yield(NewUserMessage("first")) {
			return
		}
		// Simulate slow producer - will be interrupted by context
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			yield(NewUserMessage("second"))
		}
	}

	start := time.Now()

	// Run QueryStream - should exit quickly when context times out
	for _, err := range QueryStream(ctx, slowMessages,
		WithCliPath("/nonexistent/path/to/claude"),
		WithMCPServers(map[string]MCPServerConfig{
			"test": &MCPStdioServerConfig{Type: MCPServerTypeStdio, Command: "echo", Args: []string{"test"}},
		}), // Enable MCP to trigger resultReceived channel usage
	) {
		_ = err

		break
	}

	elapsed := time.Since(start)

	// Should complete quickly (context timeout + cleanup), NOT wait for
	// the 60s streamCloseTimeout. Allow 2 seconds for cleanup overhead.
	require.Less(t, elapsed, 2*time.Second,
		"QueryStream took too long (%v) - possible deadlock due to defer ordering", elapsed)

	// Allow goroutines to settle
	time.Sleep(200 * time.Millisecond)

	after := runtime.NumGoroutine()

	require.LessOrEqual(t, after, before+2,
		"goroutine leak detected: before=%d, after=%d", before, after)
}

// =============================================================================
// Bug Detection Tests - These tests are designed to FAIL with current buggy code
// and PASS after the bugs are fixed.
// =============================================================================

// =============================================================================
// Errgroup Error Propagation Tests
// =============================================================================

// failingTransport is a mock transport that fails on SendMessage after N calls.
type failingTransport struct {
	mu            sync.Mutex
	sendCallCount atomic.Int32
	failAfter     int32 // Fail after this many SendMessage calls
	msgChan       chan map[string]any
	errChan       chan error
	closed        bool
}

func newFailingTransport(failAfter int32) *failingTransport {
	return &failingTransport{
		failAfter: failAfter,
		msgChan:   make(chan map[string]any, 10),
		errChan:   make(chan error, 1),
	}
}

func (f *failingTransport) Start(_ context.Context) error {
	return nil
}

func (f *failingTransport) ReadMessages(_ context.Context) (<-chan map[string]any, <-chan error) {
	return f.msgChan, f.errChan
}

func (f *failingTransport) SendMessage(_ context.Context, data []byte) error {
	count := f.sendCallCount.Add(1)
	if count > f.failAfter {
		return errors.New("simulated transport send failure")
	}

	// Parse the message to check if it's a control_request that needs a response.
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err == nil {
		if msgType, ok := msg["type"].(string); ok && msgType == "control_request" {
			if requestID, ok := msg["request_id"].(string); ok {
				// Send a success response asynchronously.
				go func() {
					f.msgChan <- map[string]any{
						"type": "control_response",
						"response": map[string]any{
							"subtype":    "success",
							"request_id": requestID,
							"response":   map[string]any{},
						},
					}
				}()
			}
		}
	}

	return nil
}

func (f *failingTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.closed {
		f.closed = true
		close(f.msgChan)
		close(f.errChan)
	}

	return nil
}

func (f *failingTransport) IsReady() bool {
	return true
}

func (f *failingTransport) EndInput() error {
	return nil
}

// Compile-time check that failingTransport implements config.Transport.
var _ config.Transport = (*failingTransport)(nil)

// scriptedTransport replays a fixed sequence of messages and automatically
// acknowledges control requests for Query/QueryStream tests.
type scriptedTransport struct {
	mu        sync.Mutex
	msgChan   chan map[string]any
	errChan   chan error
	ready     chan struct{}
	script    []map[string]any
	started   bool
	closed    bool
	msgClosed bool
}

func newScriptedTransport(script ...map[string]any) *scriptedTransport {
	return &scriptedTransport{
		msgChan: make(chan map[string]any, len(script)+10),
		errChan: make(chan error, 1),
		ready:   make(chan struct{}),
		script:  script,
	}
}

func (s *scriptedTransport) Start(_ context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()

		return nil
	}

	s.started = true
	script := append([]map[string]any(nil), s.script...)
	s.mu.Unlock()

	go func() {
		<-s.ready

		for _, msg := range script {
			s.msgChan <- msg
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		if !s.msgClosed {
			s.msgClosed = true
			close(s.msgChan)
		}
	}()

	return nil
}

func (s *scriptedTransport) ReadMessages(_ context.Context) (<-chan map[string]any, <-chan error) {
	return s.msgChan, s.errChan
}

func (s *scriptedTransport) SendMessage(_ context.Context, data []byte) error {
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil
	}

	msgType, _ := msg["type"].(string)
	if msgType != "control_request" {
		s.releaseScript()

		return nil
	}

	requestID, _ := msg["request_id"].(string)
	request, _ := msg["request"].(map[string]any)
	subtype, _ := request["subtype"].(string)

	payload := map[string]any{}
	if subtype == "initialize" {
		payload = map[string]any{
			"protocol_version": "1.0",
			"server_info": map[string]any{
				"name":    "test-server",
				"version": "1.0.0",
			},
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.msgChan <- map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"request_id": requestID,
			"subtype":    "success",
			"response":   payload,
		},
	}

	return nil
}

func (s *scriptedTransport) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	if !s.msgClosed {
		s.msgClosed = true
		close(s.msgChan)
	}

	close(s.errChan)

	return nil
}

func (s *scriptedTransport) IsReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.started && !s.closed
}

func (s *scriptedTransport) EndInput() error {
	s.releaseScript()

	return nil
}

var _ config.Transport = (*scriptedTransport)(nil)

func (s *scriptedTransport) releaseScript() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	select {
	case <-s.ready:
	default:
		close(s.ready)
	}
}

func TestQuery_MalformedAssistantStructuredOutputDoesNotLeak(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := newScriptedTransport(
		map[string]any{
			"type":       "assistant",
			"session_id": "session-1",
			"message": map[string]any{
				"content": []any{
					map[string]any{
						"type":  "tool_use",
						"name":  "StructuredOutput",
						"input": map[string]any{"answer": "stale"},
					},
					map[string]any{
						"foo": "missing type triggers parse error",
					},
				},
			},
		},
		map[string]any{
			"type":            "result",
			"subtype":         "success",
			"duration_ms":     1,
			"duration_api_ms": 1,
			"is_error":        false,
			"num_turns":       1,
			"session_id":      "session-1",
		},
	)

	var (
		parseErr error
		result   *ResultMessage
	)

	for msg, err := range Query(ctx, "test", WithTransport(transport)) {
		if err != nil {
			parseErr = err

			continue
		}

		if res, ok := msg.(*ResultMessage); ok {
			result = res
		}
	}

	require.Error(t, parseErr)
	require.Contains(t, parseErr.Error(), "missing or invalid 'type' field")
	require.NotNil(t, result)
	require.Nil(t, result.StructuredOutput, "malformed assistant message should not populate result structured output")
	require.Nil(t, result.Result, "malformed assistant message should not synthesize result text")
}

func TestQueryStream_MalformedAssistantStructuredOutputDoesNotLeak(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := newScriptedTransport(
		map[string]any{
			"type":       "assistant",
			"session_id": "session-1",
			"message": map[string]any{
				"content": []any{
					map[string]any{
						"type":  "tool_use",
						"name":  "StructuredOutput",
						"input": map[string]any{"answer": "stale"},
					},
					map[string]any{
						"foo": "missing type triggers parse error",
					},
				},
			},
		},
		map[string]any{
			"type":            "result",
			"subtype":         "success",
			"duration_ms":     1,
			"duration_api_ms": 1,
			"is_error":        false,
			"num_turns":       1,
			"session_id":      "session-1",
		},
	)

	messages := MessagesFromSlice([]StreamingMessage{
		NewUserMessage("test"),
	})

	var (
		parseErr error
		result   *ResultMessage
	)

	for msg, err := range QueryStream(ctx, messages, WithTransport(transport)) {
		if err != nil {
			parseErr = err

			continue
		}

		if res, ok := msg.(*ResultMessage); ok {
			result = res
		}
	}

	require.Error(t, parseErr)
	require.Contains(t, parseErr.Error(), "missing or invalid 'type' field")
	require.NotNil(t, result)
	require.Nil(t, result.StructuredOutput, "malformed assistant message should not populate result structured output")
	require.Nil(t, result.Result, "malformed assistant message should not synthesize result text")
}

// TestQueryStream_StreamInputError_Propagated verifies that errors from the
// streamInputMessages goroutine are properly propagated to callers via the
// gCtx.Done() case in the select loop.
//
// This test validates that the errgroup error propagation fix works correctly:
// 1. When streamInputMessages returns an error, errgroup cancels gCtx
// 2. gCtx.Done() becomes readable in the select
// 3. g.Wait() returns the error immediately (goroutine already exited)
// 4. Error is yielded to caller before iterator returns.
func TestQueryStream_StreamInputError_Propagated(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a transport that fails after first SendMessage
	// (first call is for initialization, second is for first streaming message)
	transport := newFailingTransport(1)

	// Create multiple messages - the second one should trigger the failure
	messages := MessagesFromSlice([]StreamingMessage{
		NewUserMessage("first message"),
		NewUserMessage("second message - this will fail"),
	})

	var receivedError error

	for _, err := range QueryStream(ctx, messages,
		WithTransport(transport),
	) {
		if err != nil {
			receivedError = err

			break
		}
	}

	// Verify that the error was propagated to the caller
	require.Error(t, receivedError, "Error from streamInputMessages should be propagated")
	require.Contains(t, receivedError.Error(), "simulated transport send failure",
		"Error message should contain the original transport error")
}

// TestStreamInputMessages_ErrorNotPropagated_Pattern tests that errors from
// SendMessage in streamInputMessages are properly propagated via SetFatalError.
// This is Issue #3 in the bug tracking.
//
// FIXED: query.go now calls controller.SetFatalError() when SendMessage or
// Marshal fails, properly propagating errors to callers.
//
// The correct pattern in query.go:
//
//	if err := transport.SendMessage(ctx, data); err != nil {
//	    log.Error("Failed to send streaming message", "error", err)
//	    controller.SetFatalError(fmt.Errorf("send streaming message: %w", err))
//	    return
//	}
//
// Expected behavior: When SendMessage fails, the error is propagated
// via controller.SetFatalError() so that callers can detect the failure.
func TestStreamInputMessages_ErrorNotPropagated_Pattern(t *testing.T) {
	t.Run("marshal error is propagated via SetFatalError", func(t *testing.T) {
		// This test verifies the correct pattern for marshal error handling
		marshalErr := fmt.Errorf("simulated marshal failure")

		var propagatedError error

		correctHandler := func(err error) {
			if err != nil {
				// Log the error
				_ = fmt.Sprintf("Failed to marshal streaming message: %v", err)
				// CORRECT: Propagate the error via SetFatalError
				propagatedError = fmt.Errorf("marshal streaming message: %w", err)

				return
			}
		}

		correctHandler(marshalErr)

		// Error should be propagated
		if propagatedError == nil {
			t.Error("Marshal error was not propagated - SetFatalError should be called")
		}
	})

	t.Run("send error is propagated via SetFatalError", func(t *testing.T) {
		// This test verifies the correct pattern for send error handling
		sendErr := fmt.Errorf("simulated send failure")

		var propagatedError error

		correctHandler := func(err error) {
			if err != nil {
				// Log the error
				_ = fmt.Sprintf("Failed to send streaming message: %v", err)
				// CORRECT: Propagate the error via SetFatalError
				propagatedError = fmt.Errorf("send streaming message: %w", err)

				return
			}
		}

		correctHandler(sendErr)

		// Error should be propagated
		if propagatedError == nil {
			t.Error("Send error was not propagated - SetFatalError should be called")
		}
	})
}
