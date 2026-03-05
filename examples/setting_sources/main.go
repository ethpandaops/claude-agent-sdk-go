package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

const systemMessageSubtypeInit = "init"

// extractSlashCommands extracts slash command names from a system message.
func extractSlashCommands(msg *claudesdk.SystemMessage) []string {
	if msg.Subtype != systemMessageSubtypeInit || msg.Data == nil {
		return nil
	}

	commands, ok := msg.Data["slash_commands"].([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(commands))

	for _, cmd := range commands {
		if cmdStr, ok := cmd.(string); ok {
			result = append(result, cmdStr)
		}
	}

	return result
}

// containsCommand checks if a command list contains a specific command.
func containsCommand(commands []string, target string) bool {
	return slices.Contains(commands, target)
}

func defaultBehavior(sdkDir string) {
	fmt.Println("=== Default Behavior Example ===")
	fmt.Println("Setting sources: None (default)")
	fmt.Println("Expected: No custom slash commands will be available")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	// No setting sources specified - isolated environment
	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithCwd(sdkDir),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "What is 2 + 2?"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		if systemMsg, ok := msg.(*claudesdk.SystemMessage); ok && systemMsg.Subtype == systemMessageSubtypeInit {
			commands := extractSlashCommands(systemMsg)
			fmt.Printf("Available slash commands: %v\n", commands)

			if containsCommand(commands, "commit") {
				fmt.Println("❌ /commit is available (unexpected)")
			} else {
				fmt.Println("✓ /commit is NOT available (expected - no settings loaded)")
			}

			break
		}

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

func userOnly(sdkDir string) {
	fmt.Println("=== User Settings Only Example ===")
	fmt.Println("Setting sources: ['user']")
	fmt.Println("Expected: Project slash commands (like /commit) will NOT be available")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithSettingSources(claudesdk.SettingSourceUser),
		claudesdk.WithCwd(sdkDir),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "What is 2 + 2?"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		if systemMsg, ok := msg.(*claudesdk.SystemMessage); ok && systemMsg.Subtype == systemMessageSubtypeInit {
			commands := extractSlashCommands(systemMsg)
			fmt.Printf("Available slash commands: %v\n", commands)

			if containsCommand(commands, "commit") {
				fmt.Println("❌ /commit is available (unexpected)")
			} else {
				fmt.Println("✓ /commit is NOT available (expected)")
			}

			break
		}

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

func multipleSources(sdkDir string) {
	fmt.Println("=== Project + User Settings Example ===")
	fmt.Println("Setting sources: ['user', 'project']")
	fmt.Println("Expected: Project slash commands (like /commit) WILL be available")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithSettingSources(
			claudesdk.SettingSourceUser,
			claudesdk.SettingSourceProject,
		),
		claudesdk.WithCwd(sdkDir),
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "What is 2 + 2?"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		if systemMsg, ok := msg.(*claudesdk.SystemMessage); ok && systemMsg.Subtype == systemMessageSubtypeInit {
			commands := extractSlashCommands(systemMsg)
			fmt.Printf("Available slash commands: %v\n", commands)

			if containsCommand(commands, "commit") {
				fmt.Println("✓ /commit is available (expected)")
			} else {
				fmt.Println("❌ /commit is NOT available (unexpected)")
			}

			break
		}

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	fmt.Println()
}

func main() {
	fmt.Println("Starting Claude SDK Setting Sources Examples...")
	fmt.Println("==================================================")
	fmt.Println()

	// Get the SDK directory (parent of examples directory)
	exePath, err := os.Executable()
	if err != nil {
		// Fall back to working directory
		exePath, _ = os.Getwd()
	}

	sdkDir := filepath.Dir(filepath.Dir(exePath))

	// If running with go run, use the source directory
	if _, err := os.Stat(filepath.Join(sdkDir, "go.mod")); os.IsNotExist(err) {
		// Try to find it relative to current working directory
		cwd, _ := os.Getwd()
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			sdkDir = cwd
		} else {
			// Go up from examples/setting_sources
			sdkDir = filepath.Join(cwd, "..", "..")
		}
	}

	examples := map[string]func(string){
		"default":          defaultBehavior,
		"user_only":        userOnly,
		"project_and_user": multipleSources,
	}

	if len(os.Args) > 1 {
		example := os.Args[1]

		if example == "all" {
			defaultBehavior(sdkDir)
			fmt.Println("--------------------------------------------------")
			fmt.Println()
			userOnly(sdkDir)
			fmt.Println("--------------------------------------------------")
			fmt.Println()
			multipleSources(sdkDir)
		} else if fn, ok := examples[example]; ok {
			fn(sdkDir)
		} else {
			fmt.Printf("Unknown example: %s\n", example)
			fmt.Println("Available: default, user_only, project_and_user, all")

			os.Exit(1)
		}
	} else {
		fmt.Println("Setting Sources Example")
		fmt.Println("\nThis example shows how to control which settings are loaded.")
		fmt.Println("\nSetting sources:")
		fmt.Println("  - user:    Global user settings (~/.claude/)")
		fmt.Println("  - project: Project-level settings (.claude/)")
		fmt.Println("  - local:   Local gitignored settings (.claude-local/)")
		fmt.Println("\nUsage:")
		fmt.Println("  go run main.go <example>")
		fmt.Println("\nExamples:")
		fmt.Println("  default          - No settings loaded (isolated)")
		fmt.Println("  user_only        - Only user settings")
		fmt.Println("  project_and_user - User + project settings")
		fmt.Println("  all              - Run all examples")
	}
}
