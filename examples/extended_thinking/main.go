// Package main demonstrates extended thinking capabilities with Claude.
//
// Extended thinking allows Claude to "think through" complex problems before
// responding, providing transparency into its reasoning process.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

// displayMessageBasic handles message display for non-streaming mode.
// It shows thinking blocks and text responses.
func displayMessageBasic(msg claudesdk.Message) {
	switch m := msg.(type) {
	case *claudesdk.AssistantMessage:
		for _, block := range m.Content {
			switch b := block.(type) {
			case *claudesdk.ThinkingBlock:
				fmt.Println("[Thinking]")
				fmt.Println(b.Thinking)
				fmt.Println("[End Thinking]")
				fmt.Println()
			case *claudesdk.TextBlock:
				fmt.Printf("Claude: %s\n", b.Text)
			}
		}

	case *claudesdk.ResultMessage:
		fmt.Println()
		fmt.Println("=== Result ===")

		if m.TotalCostUSD != nil {
			fmt.Printf("Cost: $%.6f\n", *m.TotalCostUSD)
		}
	}
}

// displayMessageStreaming handles message display for streaming mode.
// It shows thinking as it arrives in real-time.
func displayMessageStreaming(msg claudesdk.Message) {
	switch m := msg.(type) {
	case *claudesdk.StreamEvent:
		event := m.Event

		eventType, ok := event["type"].(string)
		if !ok {
			return
		}

		switch eventType {
		case "content_block_start":
			if cb, ok := event["content_block"].(map[string]any); ok {
				if cbType, ok := cb["type"].(string); ok {
					switch cbType {
					case "thinking":
						fmt.Print("[Thinking] ")
					case "text":
						fmt.Print("[Response] ")
					}
				}
			}
		case "content_block_delta":
			if delta, ok := event["delta"].(map[string]any); ok {
				if thinking, ok := delta["thinking"].(string); ok {
					fmt.Print(thinking)
				}

				if text, ok := delta["text"].(string); ok {
					fmt.Print(text)
				}
			}
		case "content_block_stop":
			fmt.Println()
		case "message_stop":
			fmt.Println()
		}

	case *claudesdk.ResultMessage:
		fmt.Println("=== Result ===")

		if m.TotalCostUSD != nil {
			fmt.Printf("Cost: $%.6f\n", *m.TotalCostUSD)
		}
	}
}

// exampleBasicThinking demonstrates extended thinking with final response.
// The thinking block is returned after Claude completes its reasoning.
func exampleBasicThinking() {
	fmt.Println("=== Basic Extended Thinking Example ===")
	fmt.Println("This example shows Claude's thinking process for a complex problem.")
	fmt.Println("Thinking is shown after completion (using WithThinking).")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithModel("claude-sonnet-4-5"),
		claudesdk.WithThinking(claudesdk.ThinkingConfigEnabled{BudgetTokens: 8000}),
		claudesdk.WithMaxTurns(1),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	prompt := "What is the sum of the first 20 prime numbers? Show your reasoning."
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Println(strings.Repeat("-", 50))

	if err := client.Query(ctx, prompt); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			fmt.Printf("Error receiving response: %v\n", err)

			return
		}

		displayMessageBasic(msg)
	}

	fmt.Println()
}

// exampleThinkingConfig demonstrates the structured ThinkingConfig API.
// Uses WithThinking for explicit control over thinking behavior.
func exampleThinkingConfig() {
	fmt.Println("=== ThinkingConfig Example ===")
	fmt.Println("This example uses WithThinking(ThinkingConfigEnabled{}) for explicit control.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	defer client.Close()

	// Use WithThinking for structured thinking configuration.
	// ThinkingConfigEnabled sets an explicit token budget.
	// ThinkingConfigAdaptive uses a default of 32,000 tokens.
	// ThinkingConfigDisabled turns off thinking entirely.
	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithModel("claude-sonnet-4-5"),
		claudesdk.WithThinking(claudesdk.ThinkingConfigEnabled{BudgetTokens: 10000}),
		claudesdk.WithEffort(claudesdk.EffortHigh),
		claudesdk.WithMaxTurns(1),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	prompt := "Explain the relationship between the Fibonacci sequence and the golden ratio."
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Println(strings.Repeat("-", 50))

	if err := client.Query(ctx, prompt); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			fmt.Printf("Error receiving response: %v\n", err)

			return
		}

		displayMessageBasic(msg)
	}

	fmt.Println()
}

// exampleStreamingThinking demonstrates real-time streaming of thinking.
// Thinking blocks are displayed as Claude generates them.
func exampleStreamingThinking() {
	fmt.Println("=== Streaming Extended Thinking Example ===")
	fmt.Println("This example shows Claude's thinking in real-time as it streams.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithModel("claude-sonnet-4-5"),
		claudesdk.WithThinking(claudesdk.ThinkingConfigEnabled{BudgetTokens: 8000}),
		claudesdk.WithIncludePartialMessages(true),
		claudesdk.WithMaxTurns(1),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	prompt := "If a train leaves Chicago at 9am traveling 60mph, and another train " +
		"leaves New York at 10am traveling 80mph toward Chicago, and they are " +
		"790 miles apart, at what time will they meet?"
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Println(strings.Repeat("-", 50))

	if err := client.Query(ctx, prompt); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		displayMessageStreaming(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

func main() {
	fmt.Println("Extended Thinking Examples")
	fmt.Println("Demonstrating Claude's reasoning transparency with thinking blocks")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
	fmt.Println("Note: Extended thinking requires a thinking-capable model")
	fmt.Println("(e.g., claude-sonnet-4-5) and WithThinking option.")
	fmt.Println()

	examples := map[string]func(){
		"basic":     exampleBasicThinking,
		"thinking":  exampleThinkingConfig,
		"streaming": exampleStreamingThinking,
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <example_name>")
		fmt.Println("\nAvailable examples:")
		fmt.Println("  all       - Run all examples")
		fmt.Println("  basic     - Show thinking after completion (WithThinking)")
		fmt.Println("  thinking  - Show thinking with ThinkingConfig and Effort")
		fmt.Println("  streaming - Stream thinking in real-time")

		return
	}

	exampleName := os.Args[1]

	if exampleName == "all" {
		exampleBasicThinking()
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println()
		exampleThinkingConfig()
		fmt.Println(strings.Repeat("-", 60))
		fmt.Println()
		exampleStreamingThinking()
	} else if fn, ok := examples[exampleName]; ok {
		fn()
	} else {
		fmt.Printf("Error: Unknown example '%s'\n", exampleName)
		fmt.Println("\nAvailable examples: all, basic, streaming")
		os.Exit(1)
	}
}
