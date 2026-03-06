package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

const systemMessageSubtypeInit = "init"

// extractAgents extracts agent names from a system message init data.
func extractAgents(msg *claudesdk.SystemMessage) []string {
	if msg.Subtype != systemMessageSubtypeInit || msg.Data == nil {
		return nil
	}

	agents, ok := msg.Data["agents"].([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(agents))

	for _, a := range agents {
		// Agents can be either strings or dicts with a 'name' field
		switch agent := a.(type) {
		case string:
			result = append(result, agent)
		case map[string]any:
			if name, ok := agent["name"].(string); ok {
				result = append(result, name)
			}
		}
	}

	return result
}

// containsAgent checks if an agent list contains a specific agent.
func containsAgent(agents []string, target string) bool {
	return slices.Contains(agents, target)
}

// displayMessage standardizes message display across examples.
func displayMessage(msg claudesdk.Message) {
	switch m := msg.(type) {
	case *claudesdk.UserMessage:
		for _, block := range m.Content.Blocks() {
			if textBlock, ok := block.(*claudesdk.TextBlock); ok {
				fmt.Printf("User: %s\n", textBlock.Text)
			}
		}

	case *claudesdk.AssistantMessage:
		for _, block := range m.Content {
			if textBlock, ok := block.(*claudesdk.TextBlock); ok {
				fmt.Printf("Claude: %s\n", textBlock.Text)
			}
		}

	case *claudesdk.SystemMessage:
		// Ignore system messages in display

	case *claudesdk.ResultMessage:
		fmt.Printf("Result: subtype=%s, cost=$%.4f\n", m.Subtype, valueOrZero(m.TotalCostUSD))
	}
}

func main() {
	fmt.Println("=== Filesystem Agents Example ===")
	fmt.Println("Testing: setting_sources=['project'] with .claude/agents/test-agent.md")
	fmt.Println()

	// Auto-detect SDK directory
	sdkDir := findSDKDir()
	fmt.Printf("SDK directory: %s\n", sdkDir)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	defer client.Close()

	fmt.Println("\nConnecting with filesystem agent loading enabled...")

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithSettingSources(claudesdk.SettingSourceProject),
		claudesdk.WithCwd(sdkDir),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	fmt.Println("Connected successfully!")

	// Query to get a response
	prompt := "Say hello in exactly 3 words"
	fmt.Printf("\nPrompt: %s\n", prompt)
	fmt.Println(strings.Repeat("-", 50))

	if err := client.Query(ctx, prompt); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	messageTypes := make([]string, 0, 10)

	var agentsFound []string

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		messageTypes = append(messageTypes, fmt.Sprintf("%T", msg))

		// Special handling for init message to extract agents
		if systemMsg, ok := msg.(*claudesdk.SystemMessage); ok && systemMsg.Subtype == systemMessageSubtypeInit {
			agentsFound = extractAgents(systemMsg)
			fmt.Printf("Init message received. Agents loaded: %v\n", agentsFound)
		}

		displayMessage(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Message types received: %v\n", messageTypes)
	fmt.Printf("Total messages: %d\n", len(messageTypes))

	// Validate the results
	// Note: reflected types show as *message.X because the public SDK exposes those message types directly.
	hasInit := containsMessageType(messageTypes, "*message.SystemMessage")
	hasAssistant := containsMessageType(messageTypes, "*message.AssistantMessage")
	hasResult := containsMessageType(messageTypes, "*message.ResultMessage")
	hasTestAgent := containsAgent(agentsFound, "test-agent")

	fmt.Println()

	if hasInit && hasAssistant && hasResult {
		fmt.Println("SUCCESS: Received full response (init, assistant, result)")
	} else {
		fmt.Println("FAILURE: Did not receive full response")
		fmt.Printf("  - Init: %v\n", hasInit)
		fmt.Printf("  - Assistant: %v\n", hasAssistant)
		fmt.Printf("  - Result: %v\n", hasResult)
	}

	if hasTestAgent {
		fmt.Println("SUCCESS: test-agent was loaded from filesystem")
	} else {
		fmt.Println("WARNING: test-agent was NOT loaded (may not exist in .claude/agents/)")
	}
}

// findSDKDir attempts to find the SDK directory.
func findSDKDir() string {
	// Try executable path first
	if exePath, err := os.Executable(); err == nil {
		sdkDir := filepath.Dir(filepath.Dir(exePath))
		if _, statErr := os.Stat(filepath.Join(sdkDir, "go.mod")); statErr == nil {
			return sdkDir
		}
	}

	// Try current working directory
	if cwd, err := os.Getwd(); err == nil {
		if _, statErr := os.Stat(filepath.Join(cwd, "go.mod")); statErr == nil {
			return cwd
		}

		// Try going up from examples/filesystem_agents
		sdkDir := filepath.Join(cwd, "..", "..")
		if _, statErr := os.Stat(filepath.Join(sdkDir, "go.mod")); statErr == nil {
			return sdkDir
		}
	}

	// Fall back to current directory
	return "."
}

// containsMessageType checks if a message type list contains a specific type.
func containsMessageType(types []string, target string) bool {
	return slices.Contains(types, target)
}

// valueOrZero returns the value of a pointer or 0 if nil.
func valueOrZero(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}

	return *ptr
}
