package main

import (
	"context"
	"fmt"
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

// singleMessageExample demonstrates QueryStream with a single message using the
// SingleMessage helper.
func singleMessageExample() {
	fmt.Println("=== Single Message Example ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// SingleMessage is the simplest way to use QueryStream for one-shot queries
	messages := claudesdk.SingleMessage("What is the capital of France?")

	for msg, err := range claudesdk.QueryStream(ctx, messages) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	fmt.Println()
}

// multiMessageExample demonstrates QueryStream with multiple messages sent as
// a batch using MessagesFromSlice.
func multiMessageExample() {
	fmt.Println("=== Multi-Message Batch Example ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// MessagesFromSlice allows sending multiple messages in sequence
	messages := claudesdk.MessagesFromSlice([]claudesdk.StreamingMessage{
		claudesdk.NewUserMessage("Hello! I have a few questions."),
		claudesdk.NewUserMessage("First, what is 2 + 2?"),
		claudesdk.NewUserMessage("Second, what is the square root of 16?"),
	})

	for msg, err := range claudesdk.QueryStream(ctx, messages) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	fmt.Println()
}

// channelExample demonstrates QueryStream with messages sent dynamically
// via a channel using MessagesFromChannel.
func channelExample() {
	fmt.Println("=== Channel-Based Dynamic Messages Example ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a channel for dynamic message generation
	msgChan := make(chan claudesdk.StreamingMessage)

	// Send messages in a goroutine to simulate dynamic input
	go func() {
		defer close(msgChan)

		questions := []string{
			"What is Go programming language?",
			"Why is it called Go?",
		}

		for _, q := range questions {
			msgChan <- claudesdk.NewUserMessage(q)

			time.Sleep(100 * time.Millisecond) // Simulate delay between messages
		}
	}()

	messages := claudesdk.MessagesFromChannel(msgChan)

	for msg, err := range claudesdk.QueryStream(ctx, messages) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	fmt.Println()
}

// withOptionsExample demonstrates QueryStream with custom options.
func withOptionsExample() {
	fmt.Println("=== QueryStream With Options Example ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	messages := claudesdk.SingleMessage("Explain what Golang is in one sentence.")

	for msg, err := range claudesdk.QueryStream(ctx, messages,
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

func main() {
	fmt.Println("QueryStream Examples")
	fmt.Println("Demonstrating iter.Seq[StreamingMessage] based streaming")
	fmt.Println()

	singleMessageExample()
	multiMessageExample()
	channelExample()
	withOptionsExample()
}
