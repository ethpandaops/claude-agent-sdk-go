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

const (
	bashToolName       = "Bash"
	writeToolName      = "Write"
	fastModelName      = "haiku"
	maxTurnsPerExample = 3
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

// examplePreToolUse demonstrates blocking commands using PreToolUse hook.
func examplePreToolUse() {
	fmt.Println("=== PreToolUse Example ===")
	fmt.Println("This example demonstrates how PreToolUse can block some bash commands but not others.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	defer client.Close()

	bashTool := bashToolName
	timeout := 5.0

	// Hook to check bash commands
	checkBashCommand := func(
		ctx context.Context,
		input claudesdk.HookInput,
		toolUseID *string,
		hookCtx *claudesdk.HookContext,
	) (claudesdk.HookJSONOutput, error) {
		preToolInput, ok := input.(*claudesdk.PreToolUseHookInput)
		if !ok {
			continueFlag := true

			return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
		}

		if preToolInput.ToolName != bashToolName {
			continueFlag := true

			return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
		}

		command, _ := preToolInput.ToolInput["command"].(string)
		blockPatterns := []string{"foo.sh"}

		for _, pattern := range blockPatterns {
			if strings.Contains(command, pattern) {
				fmt.Printf("[HOOK] Blocked command: %s\n", command)

				return &claudesdk.SyncHookJSONOutput{
					HookSpecificOutput: &claudesdk.PreToolUseHookSpecificOutput{
						HookEventName:            "PreToolUse",
						PermissionDecision:       new("deny"),
						PermissionDecisionReason: new("Command contains invalid pattern: " + pattern),
					},
				}, nil
			}
		}

		continueFlag := true

		return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
	}

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithModel(fastModelName),
		claudesdk.WithMaxTurns(maxTurnsPerExample),
		claudesdk.WithAllowedTools(bashToolName),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPreToolUse: {{
				Matcher: &bashTool,
				Hooks:   []claudesdk.HookCallback{checkBashCommand},
				Timeout: &timeout,
			}},
		}),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	// Test 1: Command with forbidden pattern (will be blocked)
	fmt.Println("Test 1: Trying a command that our PreToolUse hook should block...")
	fmt.Println("User: Run the bash command: ./foo.sh --help")

	if err := client.Query(ctx, "Run the bash command: ./foo.sh --help"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg := range client.ReceiveResponse(ctx) {
		displayMessage(msg)
	}

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Test 2: Safe command that should work
	fmt.Println("Test 2: Trying a command that our PreToolUse hook should allow...")
	fmt.Println("User: Run the bash command: echo 'Hello from hooks example!'")

	if err := client.Query(ctx, "Run the bash command: echo 'Hello from hooks example!'"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg := range client.ReceiveResponse(ctx) {
		displayMessage(msg)
	}

	fmt.Println()
}

// exampleUserPromptSubmit demonstrates adding context at user prompt submit.
func exampleUserPromptSubmit() {
	fmt.Println("=== UserPromptSubmit Example ===")
	fmt.Println("This example shows how a UserPromptSubmit hook can add context.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	timeout := 5.0

	// Hook to add custom instructions at session start
	addCustomInstructions := func(
		ctx context.Context,
		input claudesdk.HookInput,
		toolUseID *string,
		hookCtx *claudesdk.HookContext,
	) (claudesdk.HookJSONOutput, error) {
		return &claudesdk.SyncHookJSONOutput{
			HookSpecificOutput: &claudesdk.UserPromptSubmitHookSpecificOutput{
				HookEventName:     "UserPromptSubmit",
				AdditionalContext: new("My favorite color is hot pink"),
			},
		}, nil
	}

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithModel(fastModelName),
		claudesdk.WithMaxTurns(maxTurnsPerExample),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventUserPromptSubmit: {{
				Hooks:   []claudesdk.HookCallback{addCustomInstructions},
				Timeout: &timeout,
			}},
		}),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	fmt.Println("User: What's my favorite color?")

	if err := client.Query(ctx, "What's my favorite color?"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg := range client.ReceiveResponse(ctx) {
		displayMessage(msg)
	}

	fmt.Println()
}

// examplePostToolUse demonstrates reviewing tool output with reason and systemMessage.
func examplePostToolUse() {
	fmt.Println("=== PostToolUse Example ===")
	fmt.Println("This example shows how PostToolUse can provide feedback with reason and systemMessage.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	bashTool := bashToolName
	timeout := 5.0

	// Hook to review tool output
	reviewToolOutput := func(
		ctx context.Context,
		input claudesdk.HookInput,
		toolUseID *string,
		hookCtx *claudesdk.HookContext,
	) (claudesdk.HookJSONOutput, error) {
		postToolInput, ok := input.(*claudesdk.PostToolUseHookInput)
		if !ok {
			continueFlag := true

			return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
		}

		toolResponse := fmt.Sprintf("%v", postToolInput.ToolResponse)

		// If the tool produced an error, add helpful context
		if strings.Contains(strings.ToLower(toolResponse), "error") {
			return &claudesdk.SyncHookJSONOutput{
				SystemMessage: new("The command produced an error. You may want to try a different approach."),
				Reason:        new("Tool execution failed - consider checking the command syntax"),
				HookSpecificOutput: &claudesdk.PostToolUseHookSpecificOutput{
					HookEventName: "PostToolUse",
				},
			}, nil
		}

		continueFlag := true

		return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
	}

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithModel(fastModelName),
		claudesdk.WithMaxTurns(maxTurnsPerExample),
		claudesdk.WithAllowedTools(bashToolName),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPostToolUse: {{
				Matcher: &bashTool,
				Hooks:   []claudesdk.HookCallback{reviewToolOutput},
				Timeout: &timeout,
			}},
		}),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	fmt.Println("User: Run a command that will produce an error: ls /nonexistent_directory")

	if err := client.Query(ctx, "Run this command: ls /nonexistent_directory"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg := range client.ReceiveResponse(ctx) {
		displayMessage(msg)
	}

	fmt.Println()
}

// exampleDecisionFields demonstrates using permissionDecision allow/deny.
func exampleDecisionFields() {
	fmt.Println("=== Permission Decision Example ===")
	fmt.Println("This example shows how to use permissionDecision='allow'/'deny' with reason and systemMessage.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	defer client.Close()

	writeTool := writeToolName
	timeout := 5.0

	// Hook with strict approval logic
	strictApprovalHook := func(
		ctx context.Context,
		input claudesdk.HookInput,
		toolUseID *string,
		hookCtx *claudesdk.HookContext,
	) (claudesdk.HookJSONOutput, error) {
		preToolInput, ok := input.(*claudesdk.PreToolUseHookInput)
		if !ok {
			continueFlag := true

			return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
		}

		// Block any Write operations to specific files
		if preToolInput.ToolName == writeToolName {
			filePath, _ := preToolInput.ToolInput["file_path"].(string)

			if strings.Contains(strings.ToLower(filePath), "important") {
				fmt.Printf("[HOOK] Blocked Write to: %s\n", filePath)

				return &claudesdk.SyncHookJSONOutput{
					Reason:        new("Writes to files containing 'important' in the name are not allowed for safety"),
					SystemMessage: new("Write operation blocked by security policy"),
					HookSpecificOutput: &claudesdk.PreToolUseHookSpecificOutput{
						HookEventName:            "PreToolUse",
						PermissionDecision:       new("deny"),
						PermissionDecisionReason: new("Security policy blocks writes to important files"),
					},
				}, nil
			}
		}

		// Allow everything else explicitly
		return &claudesdk.SyncHookJSONOutput{
			Reason: new("Tool use approved after security review"),
			HookSpecificOutput: &claudesdk.PreToolUseHookSpecificOutput{
				HookEventName:            "PreToolUse",
				PermissionDecision:       new("allow"),
				PermissionDecisionReason: new("Tool passed security checks"),
			},
		}, nil
	}

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithTools(claudesdk.ToolsList{writeToolName}),
		claudesdk.WithModel(fastModelName),
		claudesdk.WithMaxTurns(maxTurnsPerExample),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPreToolUse: {{
				Matcher: &writeTool,
				Hooks:   []claudesdk.HookCallback{strictApprovalHook},
				Timeout: &timeout,
			}},
		}),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	// Test 1: Try to write to a file with "important" in the name (should be blocked)
	fmt.Println("Test 1: Trying to write to important_config.txt (should be blocked)...")
	fmt.Println("User: Write 'test' to important_config.txt")

	if err := client.Query(ctx, "Write the text 'test data' to a file called important_config.txt"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg := range client.ReceiveResponse(ctx) {
		displayMessage(msg)
	}

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Test 2: Write to a regular file (should be approved)
	regularFilename := fmt.Sprintf("/tmp/regular_file_%d.txt", time.Now().UnixNano())

	fmt.Printf("Test 2: Trying to write to %s (should be approved)...\n", regularFilename)
	fmt.Printf("User: Write 'test' to %s\n", regularFilename)

	if err := client.Query(ctx, fmt.Sprintf("Write the text 'test data' to the file %s", regularFilename)); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg := range client.ReceiveResponse(ctx) {
		displayMessage(msg)
	}

	fmt.Println()
}

// exampleContinueControl demonstrates using continue=false to stop execution on errors.
func exampleContinueControl() {
	fmt.Println("=== Continue/Stop Control Example ===")
	fmt.Println("This example shows how to use continue=false with stopReason to halt execution.")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	bashTool := bashToolName
	timeout := 5.0

	// Hook to stop on critical errors
	stopOnErrorHook := func(
		ctx context.Context,
		input claudesdk.HookInput,
		toolUseID *string,
		hookCtx *claudesdk.HookContext,
	) (claudesdk.HookJSONOutput, error) {
		postToolInput, ok := input.(*claudesdk.PostToolUseHookInput)
		if !ok {
			continueFlag := true

			return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
		}

		toolResponse := fmt.Sprintf("%v", postToolInput.ToolResponse)

		// Stop execution if we see a critical error
		if strings.Contains(strings.ToLower(toolResponse), "critical") {
			fmt.Println("[HOOK] Critical error detected - stopping execution")

			continueFlag := false

			return &claudesdk.SyncHookJSONOutput{
				Continue:      &continueFlag,
				StopReason:    new("Critical error detected in tool output - execution halted for safety"),
				SystemMessage: new("Execution stopped due to critical error"),
			}, nil
		}

		continueFlag := true

		return &claudesdk.SyncHookJSONOutput{Continue: &continueFlag}, nil
	}

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithModel(fastModelName),
		claudesdk.WithMaxTurns(maxTurnsPerExample),
		claudesdk.WithAllowedTools(bashToolName),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
			claudesdk.HookEventPostToolUse: {{
				Matcher: &bashTool,
				Hooks:   []claudesdk.HookCallback{stopOnErrorHook},
				Timeout: &timeout,
			}},
		}),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	fmt.Println("User: Run a command that outputs 'CRITICAL ERROR'")

	if err := client.Query(ctx, "Run this bash command: echo 'CRITICAL ERROR: system failure'"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg := range client.ReceiveResponse(ctx) {
		displayMessage(msg)
	}

	fmt.Println()
}

func main() {
	fmt.Println("Starting Claude SDK Hooks Examples...")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	examples := map[string]func(){
		"PreToolUse":       examplePreToolUse,
		"UserPromptSubmit": exampleUserPromptSubmit,
		"PostToolUse":      examplePostToolUse,
		"DecisionFields":   exampleDecisionFields,
		"ContinueControl":  exampleContinueControl,
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <example_name>")
		fmt.Println("\nAvailable examples:")
		fmt.Println("  all - Run all examples")

		for name := range examples {
			fmt.Printf("  %s\n", name)
		}

		fmt.Println("\nExample descriptions:")
		fmt.Println("  PreToolUse       - Block commands using PreToolUse hook")
		fmt.Println("  UserPromptSubmit - Add context at user prompt submit")
		fmt.Println("  PostToolUse     - Review tool output with reason and systemMessage")
		fmt.Println("  DecisionFields  - Use permissionDecision='allow'/'deny' with reason")
		fmt.Println("  ContinueControl - Control execution with continue and stopReason")

		return
	}

	exampleName := os.Args[1]

	if exampleName == "all" {
		exampleOrder := []string{
			"PreToolUse", "UserPromptSubmit", "PostToolUse",
			"DecisionFields", "ContinueControl",
		}

		for _, name := range exampleOrder {
			if fn, ok := examples[name]; ok {
				fn()
				fmt.Println(strings.Repeat("-", 50))
				fmt.Println()
			}
		}
	} else if fn, ok := examples[exampleName]; ok {
		fn()
	} else {
		fmt.Printf("Error: Unknown example '%s'\n", exampleName)
		fmt.Println("\nAvailable examples:")
		fmt.Println("  all - Run all examples")

		for name := range examples {
			fmt.Printf("  %s\n", name)
		}

		os.Exit(1)
	}
}
