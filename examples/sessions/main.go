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

// continueConversationExample demonstrates multi-turn conversation in a single session.
// Captures the session ID from the first query and uses WithResume to maintain context.
func continueConversationExample() {
	fmt.Println("=== Continue Conversation Example ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First query - establish context and capture session ID
	fmt.Println("\n--- First query: Establish context ---")

	var sessionID string

	for msg, err := range claudesdk.Query(ctx, "Remember: my favorite color is blue",
		claudesdk.WithContinueConversation(true),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			sessionID = result.SessionID
			fmt.Printf("Captured Session ID: %s\n", sessionID)
		}
	}

	if sessionID == "" {
		fmt.Println("Failed to capture session ID")

		return
	}

	// Second query - resume session to verify memory
	fmt.Println("\n--- Second query: Verify memory ---")

	for msg, err := range claudesdk.Query(ctx, "What is my favorite color?",
		claudesdk.WithResume(sessionID),
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

// resumeSessionExample demonstrates resuming a named session across separate queries.
// Uses WithResume to persist and restore session state by ID.
func resumeSessionExample() {
	fmt.Println("=== Resume Session Example ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// First query - start a new session and capture its ID
	fmt.Println("\n--- First query: Start session and establish context ---")

	var sessionID string

	for msg, err := range claudesdk.Query(ctx, "Remember: x = 42",
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)

		// Capture the session ID from the result message
		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			sessionID = result.SessionID
			fmt.Printf("Captured Session ID: %s\n", sessionID)
		}
	}

	if sessionID == "" {
		fmt.Println("Failed to capture session ID")

		return
	}

	// Second query - resume that session by ID
	fmt.Println("\n--- Second query: Resume session by ID ---")

	for msg, err := range claudesdk.Query(ctx, "What is x?",
		claudesdk.WithResume(sessionID),
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

// forkSessionExample demonstrates forking a session to explore alternatives.
// Uses WithForkSession to create a branch from an existing session.
func forkSessionExample() {
	fmt.Println("=== Fork Session Example ===")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Original session - start fresh and capture session ID
	fmt.Println("\n--- First query: Start original session ---")

	var sessionID string

	for msg, err := range claudesdk.Query(ctx, "Remember: the project language is Python",
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)

		// Capture the session ID from the result message
		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			sessionID = result.SessionID
			fmt.Printf("Captured Session ID: %s\n", sessionID)
		}
	}

	if sessionID == "" {
		fmt.Println("Failed to capture session ID")

		return
	}

	// Fork to explore alternative - creates a new session branched from original
	fmt.Println("\n--- Second query: Fork session to explore alternative ---")

	for msg, err := range claudesdk.Query(ctx, "Actually, let's use Rust instead",
		claudesdk.WithResume(sessionID),
		claudesdk.WithForkSession(true),
		claudesdk.WithMaxTurns(1),
	) {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		displayMessage(msg)
	}

	// Original session unchanged - should still remember Python
	fmt.Println("\n--- Third query: Original session unchanged (should say Python) ---")

	for msg, err := range claudesdk.Query(ctx, "What is the project language?",
		claudesdk.WithResume(sessionID),
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
	fmt.Println("Session Examples")
	fmt.Println()

	continueConversationExample()
	resumeSessionExample()
	forkSessionExample()
}
