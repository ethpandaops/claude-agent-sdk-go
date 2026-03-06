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

// displayMessage standardizes message display for partial message streaming.
func displayMessage(msg claudesdk.Message) {
	switch m := msg.(type) {
	case *claudesdk.StreamEvent:
		event := m.Event

		eventType, ok := event["type"].(string)
		if !ok {
			return
		}

		switch eventType {
		case "content_block_delta":
			if delta, ok := event["delta"].(map[string]any); ok {
				if thinking, ok := delta["thinking"].(string); ok {
					fmt.Print(thinking)
				}

				if text, ok := delta["text"].(string); ok {
					fmt.Print(text)
				}
			}
		case "message_start":
			fmt.Println("[Stream] Message started")
		case "content_block_start":
			if cb, ok := event["content_block"].(map[string]any); ok {
				if cbType, ok := cb["type"].(string); ok {
					if cbType == "thinking" {
						fmt.Print("[Thinking] ")
					}
				}
			}
		case "content_block_stop":
			fmt.Println() // Newline after block completes
		case "message_stop":
			fmt.Println("[Stream] Message completed")
		}

	case *claudesdk.UserMessage:
		for _, block := range m.Content.Blocks() {
			if textBlock, ok := block.(*claudesdk.TextBlock); ok {
				fmt.Printf("User: %s\n", textBlock.Text)
			}
		}

	case *claudesdk.AssistantMessage:
		// Skip - content already streamed via content_block_delta events

	case *claudesdk.SystemMessage:
		if m.Subtype == "init" {
			if model, ok := m.Data["model"].(string); ok {
				fmt.Printf("[System] Model: %s\n", model)
			}
		}

	case *claudesdk.ResultMessage:
		fmt.Println("Result ended")

		if m.TotalCostUSD != nil {
			fmt.Printf("Cost: $%.4f\n", *m.TotalCostUSD)
		}
	}
}

func main() {
	fmt.Println("Partial Message Streaming Example")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("\nThis feature allows you to receive stream events with incremental")
	fmt.Println("updates as Claude generates responses.")
	fmt.Println(strings.Repeat("=", 50))

	// Configure logging
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create client
	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	defer client.Close()

	// Enable partial message streaming
	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithIncludePartialMessages(true),
		claudesdk.WithModel("claude-sonnet-4-6"),
		claudesdk.WithMaxTurns(2),
		claudesdk.WithThinking(claudesdk.ThinkingConfigEnabled{BudgetTokens: 8000}),
		claudesdk.WithPermissionMode("bypassPermissions"),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	// Send a prompt that will generate a streaming response
	prompt := "Think of three jokes, then tell one"
	fmt.Printf("\nPrompt: %s\n", prompt)
	fmt.Println(strings.Repeat("=", 50))

	if err := client.Query(ctx, prompt); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	// Process messages with formatted output
	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		displayMessage(msg)

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}
}
