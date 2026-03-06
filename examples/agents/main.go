package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

// displayMessage standardizes message display across examples.
// Handles agent responses which appear as ToolResultBlocks containing TextBlocks.
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
			switch b := block.(type) {
			case *claudesdk.TextBlock:
				fmt.Printf("Claude: %s\n", b.Text)
			case *claudesdk.ToolUseBlock:
				fmt.Printf("[Agent dispatch: %s]\n", b.Name)
			case *claudesdk.ToolResultBlock:
				for _, inner := range b.Content {
					if textBlock, ok := inner.(*claudesdk.TextBlock); ok {
						fmt.Printf("Agent: %s\n", textBlock.Text)
					}
				}
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

func codeReviewerExample() {
	fmt.Println("=== Code Reviewer Agent Example ===")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithMaxTurns(2),
		claudesdk.WithAgents(map[string]*claudesdk.AgentDefinition{
			"code-reviewer": {
				Description: "Reviews code for best practices and potential issues",
				Prompt: "You are a code reviewer. Be very concise. " +
					"Give a 2-3 bullet point review only.",
				Tools: []string{"Read"},
				Model: new("claude-sonnet-4-6"),
			},
		}),
		claudesdk.WithPermissionMode("bypassPermissions"),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	prompt := "Use the code-reviewer agent to review errors.go. Be very brief, 2-3 bullets max."
	if err := client.Query(ctx, prompt); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		displayMessage(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

func documentationWriterExample() {
	fmt.Println("=== Documentation Writer Agent Example ===")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithMaxTurns(2),
		claudesdk.WithAgents(map[string]*claudesdk.AgentDefinition{
			"doc-writer": {
				Description: "Writes concise documentation",
				Prompt:      "You are a documentation expert. Be very concise.",
				Model:       new("claude-sonnet-4-6"),
			},
		}),
		claudesdk.WithPermissionMode("bypassPermissions"),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	prompt := "Use the doc-writer agent to write a one-sentence description of what a Go struct is"
	if err := client.Query(ctx, prompt); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		displayMessage(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

func multipleAgentsExample() {
	fmt.Println("=== Multiple Agents Example ===")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithMaxTurns(2),
		claudesdk.WithAgents(map[string]*claudesdk.AgentDefinition{
			"analyzer": {
				Description: "Analyzes code structure",
				Prompt:      "You are a code analyzer. Be very concise.",
			},
			"tester": {
				Description: "Creates and runs tests",
				Prompt:      "You are a testing expert. Be very concise.",
				Model:       new("claude-sonnet-4-6"),
			},
		}),
		claudesdk.WithSettingSources(
			claudesdk.SettingSourceUser,
			claudesdk.SettingSourceProject,
		),
		claudesdk.WithPermissionMode("bypassPermissions"),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	prompt := "Use the analyzer agent to explain what table-driven tests are in Go in one sentence"
	if err := client.Query(ctx, prompt); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		displayMessage(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

func main() {
	fmt.Println("Custom Agents Examples")
	fmt.Println()

	codeReviewerExample()
	documentationWriterExample()
	multipleAgentsExample()
}
