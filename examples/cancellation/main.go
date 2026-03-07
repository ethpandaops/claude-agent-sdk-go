package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

// displayMessage standardizes message display function.
func displayMessage(msg claudesdk.Message) {
	switch m := msg.(type) {
	case *claudesdk.AssistantMessage:
		for _, block := range m.Content {
			if textBlock, ok := block.(*claudesdk.TextBlock); ok {
				fmt.Printf("Claude: %s\n", textBlock.Text)
			}
		}

	case *claudesdk.ResultMessage:
		fmt.Println("Result ended")
	}
}

// exampleCancellation demonstrates cancelling a long-running hook callback.
//
// This example shows how the SDK handles cancellation of in-flight operations.
// When you interrupt (Ctrl+C), the CLI sends a control_cancel_request message
// which cancels the context passed to the hook callback.
func exampleCancellation() {
	fmt.Println("=== Cancellation Example ===")
	fmt.Println("This example demonstrates how cancellation works with hook callbacks.")
	fmt.Println("Press Ctrl+C during the long-running operation to trigger cancellation.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	defer client.Close()

	// Set up signal handling for demonstration
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n[SIGNAL] Received interrupt signal - cancellation will be handled by SDK")
		cancel()
	}()

	bashTool := "Bash"
	timeout := 30.0 // Long timeout to demonstrate cancellation

	// Hook that simulates a long-running operation
	longRunningHook := func(
		ctx context.Context,
		input claudesdk.HookInput,
		_ *string,
		_ *claudesdk.HookContext,
	) (claudesdk.HookJSONOutput, error) {
		preToolInput, ok := input.(*claudesdk.PreToolUseHookInput)
		if !ok {
			continueFlag := true

			return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
		}

		fmt.Printf("[HOOK] Starting long-running check for tool: %s\n", preToolInput.ToolName)
		fmt.Println("[HOOK] Simulating work... (Press Ctrl+C to cancel)")

		// Simulate a long-running operation that checks for cancellation
		for i := 1; i <= 10; i++ {
			select {
			case <-ctx.Done():
				fmt.Printf("[HOOK] Operation cancelled after %d seconds!\n", i-1)
				fmt.Printf("[HOOK] Cancellation reason: %v\n", ctx.Err())

				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
				fmt.Printf("[HOOK] Working... %d/10 seconds\n", i)
			}
		}

		fmt.Println("[HOOK] Operation completed successfully")

		continueFlag := true

		return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
	}

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithAllowedTools("Bash"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPreToolUse: {{
				Matcher: &bashTool,
				Hooks:   []claudesdk.HookCallback{longRunningHook},
				Timeout: &timeout,
			}},
		}),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	fmt.Println("User: Run a simple echo command")
	fmt.Println()

	if err := client.Query(ctx, "Run this bash command: echo 'Hello World'"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg := range client.ReceiveResponse(ctx) {
		displayMessage(msg)
	}

	fmt.Println()
}

// exampleGracefulShutdown demonstrates graceful shutdown with in-flight operations.
//
// This example shows how calling client.Close() will cancel all in-flight
// operations and wait for them to complete gracefully.
func exampleGracefulShutdown() {
	fmt.Println("=== Graceful Shutdown Example ===")
	fmt.Println("This example demonstrates graceful shutdown of in-flight operations.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	bashTool := "Bash"
	timeout := 30.0

	// Track hook execution
	hookStarted := make(chan struct{})

	// Hook that waits for cancellation
	waitingHook := func(
		ctx context.Context,
		input claudesdk.HookInput,
		_ *string,
		_ *claudesdk.HookContext,
	) (claudesdk.HookJSONOutput, error) {
		preToolInput, ok := input.(*claudesdk.PreToolUseHookInput)
		if !ok {
			continueFlag := true

			return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
		}

		fmt.Printf("[HOOK] Hook started for tool: %s\n", preToolInput.ToolName)
		close(hookStarted)

		// Wait for cancellation
		<-ctx.Done()
		fmt.Println("[HOOK] Context cancelled during graceful shutdown")

		return nil, ctx.Err()
	}

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithAllowedTools("Bash"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPreToolUse: {{
				Matcher: &bashTool,
				Hooks:   []claudesdk.HookCallback{waitingHook},
				Timeout: &timeout,
			}},
		}),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	// Start query in background
	go func() {
		if err := client.Query(ctx, "Run: echo test"); err != nil {
			fmt.Printf("Query error (expected during shutdown): %v\n", err)
		}
	}()

	// Wait for hook to start
	select {
	case <-hookStarted:
		fmt.Println("[MAIN] Hook is running, initiating graceful shutdown...")
	case <-time.After(15 * time.Second):
		fmt.Println("[MAIN] Timeout waiting for hook to start")

		return
	}

	// Give a moment for things to settle
	time.Sleep(500 * time.Millisecond)

	// Close client - this should cancel all in-flight operations
	fmt.Println("[MAIN] Calling client.Close() - this will cancel in-flight operations")

	if err := client.Close(); err != nil {
		fmt.Printf("[MAIN] Close completed with: %v\n", err)
	} else {
		fmt.Println("[MAIN] Close completed successfully")
	}

	fmt.Println()
}

func main() {
	fmt.Println("Starting Claude SDK Cancellation Examples...")
	fmt.Println("============================================")
	fmt.Println()

	examples := map[string]func(){
		"cancellation":      exampleCancellation,
		"graceful_shutdown": exampleGracefulShutdown,
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <example_name>")
		fmt.Println("\nAvailable examples:")
		fmt.Println("  all               - Run all examples")
		fmt.Println("  cancellation      - Demonstrate cancelling a long-running hook (press Ctrl+C)")
		fmt.Println("  graceful_shutdown - Demonstrate graceful shutdown of in-flight operations")

		return
	}

	exampleName := os.Args[1]

	if exampleName == "all" {
		exampleOrder := []string{"cancellation", "graceful_shutdown"}

		for _, name := range exampleOrder {
			if fn, ok := examples[name]; ok {
				fn()
				fmt.Println("--------------------------------------------------")
				fmt.Println()
			}
		}
	} else if fn, ok := examples[exampleName]; ok {
		fn()
	} else {
		fmt.Printf("Unknown example: %s\n", exampleName)
		fmt.Println("\nAvailable examples:")
		fmt.Println("  all               - Run all examples")

		for name := range examples {
			fmt.Printf("  %s\n", name)
		}

		os.Exit(1)
	}
}
