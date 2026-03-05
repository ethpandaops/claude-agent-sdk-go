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
				fmt.Printf("Response: %s\n", textBlock.Text)
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

func main() {
	fmt.Println("Stderr Callback Example")
	fmt.Println("Capturing CLI debug output via stderr callback")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	// Collect stderr messages
	var stderrMessages []string

	// Create stderr callback
	stderrCallback := func(message string) {
		stderrMessages = append(stderrMessages, message)

		// Optionally print specific messages
		if strings.Contains(message, "[ERROR]") {
			fmt.Printf("Error detected: %s\n", message)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	// Enable debug output to stderr
	debugFlag := ""

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithStderr(stderrCallback),
		claudesdk.WithExtraArgs(map[string]*string{
			"debug-to-stderr": &debugFlag, // Enable debug output
		}),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	fmt.Println("Running query with stderr capture...")

	if err := client.Query(ctx, "What is 2+2?"); err != nil {
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

	// Show what we captured
	fmt.Printf("\nCaptured %d stderr lines\n", len(stderrMessages))

	if len(stderrMessages) > 0 {
		firstLine := stderrMessages[0]
		if len(firstLine) > 100 {
			firstLine = firstLine[:100]
		}

		fmt.Printf("First stderr line: %s\n", firstLine)
	}
}
