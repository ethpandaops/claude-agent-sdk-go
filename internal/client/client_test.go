package client

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/userinput"
)

// mockTransport implements config.Transport for testing.
// It automatically responds to initialize control requests.
type mockTransport struct {
	mu       sync.Mutex
	started  bool
	closed   bool
	messages chan map[string]any
	errors   chan error
	sent     []map[string]any
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		messages: make(chan map[string]any, 100),
		errors:   make(chan error, 10),
	}
}

func (m *mockTransport) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.started = true

	return nil
}

func (m *mockTransport) ReadMessages(_ context.Context) (<-chan map[string]any, <-chan error) {
	return m.messages, m.errors
}

func (m *mockTransport) SendMessage(_ context.Context, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse the message to check if it's an init request
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil
	}

	m.sent = append(m.sent, msg)

	// Auto-respond to control_request messages.
	if msgType, _ := msg["type"].(string); msgType == "control_request" {
		requestID, _ := msg["request_id"].(string)

		request, _ := msg["request"].(map[string]any)
		subtype, _ := request["subtype"].(string)

		go func(subtype string) {
			m.mu.Lock()
			defer m.mu.Unlock()

			if m.closed {
				return
			}

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

			m.messages <- map[string]any{
				"type": "control_response",
				"response": map[string]any{
					"request_id": requestID,
					"subtype":    "success",
					"response":   payload,
				},
			}
		}(subtype)
	}

	return nil
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		m.closed = true
		close(m.messages)
		close(m.errors)
	}

	return nil
}

func (m *mockTransport) IsReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.started && !m.closed
}

func (m *mockTransport) EndInput() error {
	return nil
}

// TestClient_StartContextCancellation tests that the client's errgroup
// uses context.Background() rather than the caller's context.
//
// This verifies the fix for the errgroup context issue where the errgroup
// was tied to the caller's ctx, causing the client to stop when ctx expired.
//
// Note: Full message reception after ctx cancellation also depends on the
// protocol controller's context handling, which is outside the scope of this fix.
func TestClient_StartContextCancellation(t *testing.T) {
	t.Run("client remains connected after startup context cancelled", func(t *testing.T) {
		// Create a context that we'll cancel after Start() returns
		ctx, cancel := context.WithCancel(context.Background())

		transport := newMockTransport()

		client := New()

		err := client.Start(ctx, &config.Options{
			Transport: transport,
		})
		require.NoError(t, err)

		// Client should be connected
		assert.True(t, client.isConnected(), "client should be connected after Start()")

		// Cancel the startup context
		cancel()

		// Give time for cancellation to propagate
		time.Sleep(50 * time.Millisecond)

		// Client should still be marked as connected
		// (the connected flag is not affected by ctx cancellation)
		assert.True(t, client.isConnected(), "client should remain connected after ctx cancel")

		// Clean up
		err = client.Close()
		require.NoError(t, err)
	})

	t.Run("client remains connected after startup context timeout", func(t *testing.T) {
		// Create a context with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		transport := newMockTransport()

		client := New()

		err := client.Start(ctx, &config.Options{
			Transport: transport,
		})
		require.NoError(t, err)

		// Wait for the timeout to expire
		time.Sleep(250 * time.Millisecond)

		// Client should still be marked as connected
		assert.True(t, client.isConnected(), "client should remain connected after ctx timeout")

		// Clean up
		err = client.Close()
		require.NoError(t, err)
	})
}

func TestClient_StartWithOnUserInputAutoConfiguresPermissionPrompt(t *testing.T) {
	transport := newMockTransport()

	client := New()

	err := client.Start(context.Background(), &config.Options{
		Transport: transport,
		OnUserInput: func(
			_ context.Context,
			_ *userinput.Request,
		) (*userinput.Response, error) {
			return &userinput.Response{}, nil
		},
	})
	require.NoError(t, err)

	require.NotNil(t, client.options)
	require.Equal(t, "stdio", client.options.PermissionPromptToolName)
	require.NoError(t, client.Close())
}

func TestClient_StartWithOnUserInputAndPermissionPromptToolNameReturnsError(t *testing.T) {
	transport := newMockTransport()

	client := New()

	err := client.Start(context.Background(), &config.Options{
		Transport:                transport,
		PermissionPromptToolName: "custom",
		OnUserInput: func(
			_ context.Context,
			_ *userinput.Request,
		) (*userinput.Response, error) {
			return &userinput.Response{}, nil
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "on_user_input callback cannot be used with permission_prompt_tool_name")
}

func TestClientMCPControlRequests(t *testing.T) {
	transport := newMockTransport()
	client := New()

	err := client.Start(context.Background(), &config.Options{Transport: transport})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	require.NoError(t, client.ReconnectMCPServer(context.Background(), "my-server"))
	require.NoError(t, client.ToggleMCPServer(context.Background(), "my-server", false))
	require.NoError(t, client.StopTask(context.Background(), "task-123"))

	transport.mu.Lock()
	defer transport.mu.Unlock()

	require.Len(t, transport.sent, 4)

	var subtypes []string

	for _, sent := range transport.sent {
		if sent["type"] != "control_request" {
			continue
		}

		request, _ := sent["request"].(map[string]any)
		subtype, _ := request["subtype"].(string)
		subtypes = append(subtypes, subtype)
	}

	require.Contains(t, subtypes, "initialize")
	require.Contains(t, subtypes, "mcp_reconnect")
	require.Contains(t, subtypes, "mcp_toggle")
	require.Contains(t, subtypes, "stop_task")

	last := transport.sent[len(transport.sent)-1]
	request, _ := last["request"].(map[string]any)
	require.Equal(t, "task-123", request["task_id"])
}

// TestClient_DoneChannelStopsReadLoop verifies that the c.done channel
// properly signals shutdown to the readLoop, independent of context.
func TestClient_DoneChannelStopsReadLoop(t *testing.T) {
	transport := newMockTransport()

	client := New()

	err := client.Start(context.Background(), &config.Options{
		Transport: transport,
	})
	require.NoError(t, err)

	// Client is connected
	assert.True(t, client.isConnected())

	// Close should properly signal done and stop the readLoop
	err = client.Close()
	require.NoError(t, err)

	// Client should be disconnected
	assert.False(t, client.isConnected())
}

// TestClient_QueryAfterContextCancel verifies that Query works after
// the startup context is cancelled.
func TestClient_QueryAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	transport := newMockTransport()

	client := New()

	err := client.Start(ctx, &config.Options{
		Transport: transport,
	})
	require.NoError(t, err)

	// Cancel startup context
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Query should still work with a fresh context
	// (Query only sends via transport, doesn't depend on readLoop)
	queryCtx := context.Background()
	err = client.Query(queryCtx, "test query")
	require.NoError(t, err)

	// Clean up
	err = client.Close()
	require.NoError(t, err)
}

// TestClient_CloseAfterContextCancel verifies that Close works correctly
// after the startup context is cancelled.
func TestClient_CloseAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	transport := newMockTransport()

	client := New()

	err := client.Start(ctx, &config.Options{
		Transport: transport,
	})
	require.NoError(t, err)

	// Cancel startup context
	cancel()

	// Close should work without hanging or panicking
	err = client.Close()
	require.NoError(t, err)

	// Verify client is no longer connected
	assert.False(t, client.isConnected())
}

// TestClient_ErrGroupDoesNotExitOnContextCancel verifies that the errgroup
// goroutines don't immediately exit when the startup context is cancelled.
// This is the core behavior fixed by using context.Background() for errgroup.
func TestClient_ErrGroupDoesNotExitOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	transport := newMockTransport()

	client := New()

	err := client.Start(ctx, &config.Options{
		Transport: transport,
	})
	require.NoError(t, err)

	// At this point, the errgroup is running with context.Background()
	// Cancelling the startup ctx should NOT cause the errgroup to fail

	// Cancel the startup context
	cancel()

	// Give time for any cancellation to propagate
	time.Sleep(100 * time.Millisecond)

	// Verify the errgroup hasn't returned an error by checking that
	// we can still close cleanly (eg.Wait() is called in Close())
	err = client.Close()
	require.NoError(t, err)
}
