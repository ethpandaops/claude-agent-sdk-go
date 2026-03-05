package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

const systemMessageSubtypeInit = "init"

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
		fmt.Println("Result ended")

		if m.TotalCostUSD != nil {
			fmt.Printf("Cost: $%.4f\n", *m.TotalCostUSD)
		}
	}
}

// extractTools extracts tool names from a system message.
func extractTools(msg *claudesdk.SystemMessage) []string {
	if msg.Subtype != systemMessageSubtypeInit || msg.Data == nil {
		return nil
	}

	tools, ok := msg.Data["tools"].([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(tools))

	for _, tool := range tools {
		if toolStr, ok := tool.(string); ok {
			result = append(result, toolStr)
		}
	}

	return result
}

// toolsArrayExample demonstrates restricting tools to a specific array.
func toolsArrayExample() {
	fmt.Println("=== Tools Array Example ===")
	fmt.Println("Setting Tools=['Read', 'Glob', 'Grep']")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithTools(claudesdk.ToolsList{"Read", "Glob", "Grep"}),
		claudesdk.WithMaxTurns(1),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "What tools do you have available? Just list them briefly."); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		// Special handling for init message to show tools
		if systemMsg, ok := msg.(*claudesdk.SystemMessage); ok && systemMsg.Subtype == systemMessageSubtypeInit {
			tools := extractTools(systemMsg)
			fmt.Printf("Tools from system message: %v\n", tools)
			fmt.Println()
		}

		displayMessage(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

// toolsSingleToolExample demonstrates restricting to a single tool.
func toolsSingleToolExample() {
	fmt.Println("=== Tools Single Tool Example ===")
	fmt.Println("Setting Tools=['Read'] (only Read available)")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithTools(claudesdk.ToolsList{"Read"}),
		claudesdk.WithMaxTurns(1),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "What tools do you have available? Just list them briefly."); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		// Special handling for init message to show tools
		if systemMsg, ok := msg.(*claudesdk.SystemMessage); ok && systemMsg.Subtype == systemMessageSubtypeInit {
			tools := extractTools(systemMsg)
			fmt.Printf("Tools from system message: %v\n", tools)
			fmt.Println()
		}

		displayMessage(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

// toolsPresetExample demonstrates using a preset configuration.
func toolsPresetExample() {
	fmt.Println("=== Tools Preset Example ===")
	fmt.Println("Setting Tools={type: 'preset', preset: 'claude_code'}")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithTools(&claudesdk.ToolsPreset{Type: "preset", Preset: "claude_code"}),
		claudesdk.WithMaxTurns(1),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "What tools do you have available? Just list them briefly."); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		// Special handling for init message to show tools
		if systemMsg, ok := msg.(*claudesdk.SystemMessage); ok && systemMsg.Subtype == systemMessageSubtypeInit {
			tools := extractTools(systemMsg)

			if len(tools) > 5 {
				fmt.Printf("Tools from system message (%d tools): %v...\n", len(tools), tools[:5])
			} else {
				fmt.Printf("Tools from system message (%d tools): %v\n", len(tools), tools)
			}

			fmt.Println()
		}

		displayMessage(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

func main() {
	fmt.Println("Tools Option Examples")
	fmt.Println()
	fmt.Println("This example demonstrates how to control which tools are available.")
	fmt.Println()

	examples := map[string]func(){
		"array":  toolsArrayExample,
		"single": toolsSingleToolExample,
		"preset": toolsPresetExample,
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <example_name>")
		fmt.Println("\nAvailable examples:")
		fmt.Println("  array  - Restrict to specific tools (Read, Glob, Grep)")
		fmt.Println("  single - Restrict to a single tool (Read)")
		fmt.Println("  preset - Use claude_code preset for all default tools")

		return
	}

	exampleName := os.Args[1]

	if exampleName == "all" {
		for _, name := range []string{"array", "single", "preset"} {
			examples[name]()
			fmt.Println("--------------------------------------------------")
			fmt.Println()
		}
	} else if fn, ok := examples[exampleName]; ok {
		fn()
	} else {
		fmt.Printf("Error: Unknown example '%s'\n", exampleName)
		fmt.Println("\nAvailable examples:")
		fmt.Println("  array  - Restrict to specific tools")
		fmt.Println("  single - Restrict to a single tool")
		fmt.Println("  preset - Use claude_code preset")
		fmt.Println("  all    - Run all examples")

		os.Exit(1)
	}
}
