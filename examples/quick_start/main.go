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

// basicExample demonstrates a simple question.
func basicExample() {
	fmt.Println("=== Basic Example ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for msg, err := range claudesdk.Query(ctx, "What is 2 + 2?") {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	fmt.Println()
}

// withOptionsExample demonstrates using custom options.
func withOptionsExample() {
	fmt.Println("=== With Options Example ===")

	_ = slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for msg, err := range claudesdk.Query(ctx, "Explain what Golang is in one sentence.",
		claudesdk.WithSystemPrompt("You are a helpful assistant that explains things simply."),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	fmt.Println()
}

// withToolsExample demonstrates using allowed tools with cost reporting.
func withToolsExample() {
	fmt.Println("=== With Tools Example ===")

	_ = slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for msg, err := range claudesdk.Query(ctx, "Create a file called hello.txt with 'Hello, World!' in it",
		claudesdk.WithAllowedTools("Read", "Write"),
		claudesdk.WithSystemPrompt("You are a helpful file assistant."),
	) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	fmt.Println()
}

func main() {
	fmt.Println("Quick Start Examples")
	fmt.Println()

	basicExample()
	withOptionsExample()
	withToolsExample()
}
