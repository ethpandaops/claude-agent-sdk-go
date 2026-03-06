//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
	"github.com/stretchr/testify/require"
)

// TestAgentsAndSettings_AgentDefinition tests custom agent configuration.
func TestAgentsAndSettings_AgentDefinition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sonnetModel := "sonnet"
	receivedResponse := false

	for msg, err := range claudesdk.Query(ctx, "Say 'hello'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithAgents(map[string]*claudesdk.AgentDefinition{
			"test-agent": {
				Description: "A test agent for unit testing",
				Prompt:      "You are a helpful test agent. When asked, reply with 'TEST_AGENT_OK'.",
				Tools:       []string{"Read", "Grep"},
				Model:       &sonnetModel,
			},
		}),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			t.Logf("Received assistant message with %d content blocks", len(m.Content))
			receivedResponse = true
		case *claudesdk.ResultMessage:
			require.False(t, m.IsError, "Query should not result in error")
		}
	}

	require.True(t, receivedResponse, "Should receive assistant response")
}

// TestAgentsAndSettings_SettingSources tests setting source loading.
func TestAgentsAndSettings_SettingSources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	receivedResult := false

	for msg, err := range claudesdk.Query(ctx, "What is 2+2? Reply with just the number.",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithSettingSources(claudesdk.SettingSourceUser, claudesdk.SettingSourceProject),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			receivedResult = true
			require.False(t, result.IsError, "Query should not result in error")
		}
	}

	require.True(t, receivedResult, "Should receive result message")
}

// TestAgentsAndSettings_NoSettingSources tests isolated environment without settings.
func TestAgentsAndSettings_NoSettingSources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	receivedResult := false

	for msg, err := range claudesdk.Query(ctx, "Say 'isolated'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithSettingSources(),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			receivedResult = true
			require.False(t, result.IsError, "Query should not result in error")
		}
	}

	require.True(t, receivedResult, "Should receive result message")
}

// TestAgentsAndSettings_FilesystemAgentLoading tests that filesystem-based agents
// load via setting_sources=["project"] and produce a full response cycle.
func TestAgentsAndSettings_FilesystemAgentLoading(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create a temporary project directory
	tmpDir, err := os.MkdirTemp("", "claude-sdk-test-*")
	require.NoError(t, err)

	defer os.RemoveAll(tmpDir)

	// Create .claude/agents directory
	agentsDir := filepath.Join(tmpDir, ".claude", "agents")
	err = os.MkdirAll(agentsDir, 0755)
	require.NoError(t, err)

	// Create a test agent file with YAML frontmatter
	agentFile := filepath.Join(agentsDir, "fs-test-agent.md")
	agentContent := `---
name: fs-test-agent
description: A filesystem test agent for SDK testing
tools: Read
---

# Filesystem Test Agent

You are a simple test agent. When asked a question, provide a brief, helpful answer.
`
	err = os.WriteFile(agentFile, []byte(agentContent), 0644)
	require.NoError(t, err)

	var (
		receivedSystem    bool
		receivedAssistant bool
		receivedResult    bool
		foundAgent        bool
	)

	for msg, err := range claudesdk.Query(ctx, "Say hello in exactly 3 words",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithSettingSources(claudesdk.SettingSourceProject),
		claudesdk.WithCwd(tmpDir),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.SystemMessage:
			receivedSystem = true

			if m.Subtype == "init" {
				if agents, ok := m.Data["agents"].([]any); ok {
					for _, agent := range agents {
						if agentName, ok := agent.(string); ok && agentName == "fs-test-agent" {
							foundAgent = true

							t.Logf("Found filesystem agent: %s", agentName)
						}
					}
				}
			}
		case *claudesdk.AssistantMessage:
			receivedAssistant = true
			t.Logf("Received assistant message with %d content blocks", len(m.Content))
		case *claudesdk.ResultMessage:
			receivedResult = true
			require.False(t, m.IsError, "Query should not result in error")
		}
	}

	require.True(t, receivedSystem, "Should receive SystemMessage (init)")
	require.True(t, receivedAssistant,
		"Should receive AssistantMessage - missing may indicate filesystem agent loading issue")
	require.True(t, receivedResult, "Should receive ResultMessage")
	require.True(t, foundAgent,
		"fs-test-agent should be loaded from filesystem via setting_sources")
}
