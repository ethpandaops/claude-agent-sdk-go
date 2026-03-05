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
		if m.TotalCostUSD != nil {
			fmt.Printf("Total cost: $%.4f\n", *m.TotalCostUSD)
		}

		fmt.Printf("Status: %s\n", m.Subtype)

		if m.Subtype == "error_max_budget_usd" {
			fmt.Println("Budget limit exceeded!")
			fmt.Println("Note: The cost may exceed the budget by up to one API call's worth")
		}
	}
}

func withoutBudget() {
	fmt.Println("=== Without Budget Limit ===")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx, claudesdk.WithLogger(logger)); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "What is 2 + 2?"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			fmt.Printf("Failed to receive response: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	fmt.Println()
}

func withReasonableBudget() {
	fmt.Println("=== With Reasonable Budget ($0.10) ===")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	// 10 cents - plenty for a simple query
	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithMaxBudgetUSD(0.10),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "What is 2 + 2?"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			fmt.Printf("Failed to receive response: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	fmt.Println()
}

func withTightBudget() {
	fmt.Println("=== With Tight Budget ($0.0001) ===")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	// Very small budget - will be exceeded quickly
	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithMaxBudgetUSD(0.0001),
		claudesdk.WithPermissionMode("bypassPermissions"),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	queryText := "Read the README.md file and summarize it"
	if err := client.Query(ctx, queryText); err != nil {
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
	fmt.Println("Max Budget USD Examples")
	fmt.Println()
	fmt.Println("This example demonstrates using max_budget_usd to control API costs.")
	fmt.Println()

	withoutBudget()
	withReasonableBudget()
	withTightBudget()

	fmt.Println("\nNote: Budget checking happens after each API call completes,")
	fmt.Println("so the final cost may slightly exceed the specified budget.")
}
