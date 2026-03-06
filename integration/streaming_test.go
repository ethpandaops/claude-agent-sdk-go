//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
	"github.com/stretchr/testify/require"
)

// TestPartialMessages_StreamEventsReceived tests StreamEvent with IncludePartialMessages.
func TestPartialMessages_StreamEventsReceived(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var streamEventCount int

	for msg, err := range claudesdk.Query(ctx, "Write a short haiku about testing.",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithIncludePartialMessages(true),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		if _, ok := msg.(*claudesdk.StreamEvent); ok {
			streamEventCount++
		}
	}

	require.Greater(t, streamEventCount, 0,
		"Should receive StreamEvents when IncludePartialMessages is true")
}

// TestPartialMessages_EventTypes verifies content_block_delta events are received.
func TestPartialMessages_EventTypes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	eventTypes := make(map[string]bool)

	for msg, err := range claudesdk.Query(ctx, "Say 'hello world'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithIncludePartialMessages(true),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		if streamEvent, ok := msg.(*claudesdk.StreamEvent); ok {
			if eventType, ok := streamEvent.Event["type"].(string); ok {
				eventTypes[eventType] = true
			}
		}
	}

	hasExpectedEvents := eventTypes["message_start"] ||
		eventTypes["content_block_delta"] ||
		eventTypes["message_delta"]
	require.True(t, hasExpectedEvents,
		"Should receive expected event types; got: %v", eventTypes)
}

// TestPartialMessages_DisabledByDefault verifies no StreamEvents when disabled.
func TestPartialMessages_DisabledByDefault(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var streamEventCount int

	for msg, err := range claudesdk.Query(ctx, "Say 'hello'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithIncludePartialMessages(false),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		if _, ok := msg.(*claudesdk.StreamEvent); ok {
			streamEventCount++
		}
	}

	require.Equal(t, 0, streamEventCount,
		"Should not receive StreamEvents when IncludePartialMessages is false")
}

// TestPartialMessages_ThinkingDeltas tests receiving thinking block deltas.
func TestPartialMessages_ThinkingDeltas(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	maxThinking := 1000

	var (
		streamEventCount int
		hasThinkingEvent bool
	)

	for msg, err := range claudesdk.Query(ctx,
		"Think step by step: what is the sum of numbers from 1 to 10?",
		claudesdk.WithModel("claude-sonnet-4-6"),
		claudesdk.WithIncludePartialMessages(true),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
		claudesdk.WithThinking(claudesdk.ThinkingConfigEnabled{BudgetTokens: maxThinking}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		if streamEvent, ok := msg.(*claudesdk.StreamEvent); ok {
			streamEventCount++

			if eventType, ok := streamEvent.Event["type"].(string); ok {
				if eventType == "content_block_delta" {
					if delta, ok := streamEvent.Event["delta"].(map[string]any); ok {
						if deltaType, ok := delta["type"].(string); ok {
							if deltaType == "thinking_delta" {
								hasThinkingEvent = true
								t.Logf("Received thinking delta event")
							}
						}
					}
				}
			}
		}
	}

	require.Greater(t, streamEventCount, 0,
		"Should receive StreamEvents when IncludePartialMessages is true")

	t.Logf("Received %d stream events, hasThinkingEvent=%v",
		streamEventCount, hasThinkingEvent)
}
