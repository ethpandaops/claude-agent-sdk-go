// Package main demonstrates how to use plugins with Claude Code SDK.
//
// Plugins allow you to extend Claude Code with custom commands, agents, skills,
// and hooks. This example shows how to load a local plugin and verify it's
// loaded by checking the system message.
//
// The demo plugin is located in examples/plugins/demo-plugin/ and provides
// a custom /greet command.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

const systemMessageSubtypeInit = "init"

// extractPlugins extracts plugin info from a system message.
func extractPlugins(msg *claudesdk.SystemMessage) []map[string]any {
	if msg.Subtype != systemMessageSubtypeInit || msg.Data == nil {
		return nil
	}

	plugins, ok := msg.Data["plugins"].([]any)
	if !ok {
		return nil
	}

	result := make([]map[string]any, 0, len(plugins))

	for _, p := range plugins {
		if plugin, ok := p.(map[string]any); ok {
			result = append(result, plugin)
		}
	}

	return result
}

func pluginExample() {
	fmt.Println("=== Plugin Example ===")
	fmt.Println()

	// Auto-detect the plugin path
	pluginPath := findPluginPath()
	fmt.Printf("Loading plugin from: %s\n\n", pluginPath)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer client.Close()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithPlugins(&claudesdk.SdkPluginConfig{
			Type: "local",
			Path: pluginPath,
		}),
		claudesdk.WithMaxTurns(1), // Limit to one turn for quick demo
	); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)

		return
	}

	if err := client.Query(ctx, "Hello!"); err != nil {
		fmt.Printf("Failed to send query: %v\n", err)

		return
	}

	foundPlugins := false

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			break
		}

		if systemMsg, ok := msg.(*claudesdk.SystemMessage); ok && systemMsg.Subtype == systemMessageSubtypeInit {
			fmt.Println("System initialized!")

			if systemMsg.Data != nil {
				keys := make([]string, 0, len(systemMsg.Data))
				for k := range systemMsg.Data {
					keys = append(keys, k)
				}

				fmt.Printf("System message data keys: %v\n\n", keys)
			}

			// Check for plugins in the system message
			pluginsData := extractPlugins(systemMsg)

			if len(pluginsData) > 0 {
				fmt.Println("Plugins loaded:")

				for _, plugin := range pluginsData {
					name, _ := plugin["name"].(string)
					path, _ := plugin["path"].(string)
					fmt.Printf("  - %s (path: %s)\n", name, path)
				}

				foundPlugins = true
			} else {
				fmt.Println("Note: Plugin was passed via CLI but may not appear in system message.")
				fmt.Printf("Plugin path configured: %s\n", pluginPath)

				foundPlugins = true
			}
		}

		if _, ok := msg.(*claudesdk.ResultMessage); ok {
			break
		}
	}

	if foundPlugins {
		fmt.Println()
		fmt.Println("Plugin successfully configured!")
		fmt.Println()
	}
}

// findPluginPath finds the demo plugin path.
func findPluginPath() string {
	// Try relative to current working directory
	if cwd, err := os.Getwd(); err == nil {
		// Direct path from repo root
		path := filepath.Join(cwd, "examples", "plugins", "demo-plugin")
		if _, statErr := os.Stat(path); statErr == nil {
			return path
		}

		// If running from examples/plugin_example directory
		path = filepath.Join(cwd, "..", "plugins", "demo-plugin")
		if _, statErr := os.Stat(path); statErr == nil {
			return path
		}
	}

	// Try relative to executable
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		path := filepath.Join(exeDir, "..", "plugins", "demo-plugin")

		if _, statErr := os.Stat(path); statErr == nil {
			return path
		}
	}

	// Fall back to a relative path
	return "./examples/plugins/demo-plugin"
}

func main() {
	pluginExample()
}
