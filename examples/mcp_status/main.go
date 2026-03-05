// Package main demonstrates querying MCP server connection status.
//
// This example creates an in-process MCP server, starts a client with it
// configured, and queries the live connection status of all MCP servers.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create a simple calculator MCP server with one tool.
	addTool := claudesdk.NewSdkMcpTool(
		"add",
		"Add two numbers",
		claudesdk.SimpleSchema(map[string]string{"a": "float64", "b": "float64"}),
		func(_ context.Context, req *claudesdk.CallToolRequest) (*claudesdk.CallToolResult, error) {
			args, err := claudesdk.ParseArguments(req)
			if err != nil {
				return claudesdk.ErrorResult(err.Error()), nil
			}

			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)

			return claudesdk.TextResult(fmt.Sprintf("%v + %v = %v", a, b, a+b)), nil
		},
	)

	calculator := claudesdk.CreateSdkMcpServer("calc", "1.0.0", addTool)

	// Start client with the MCP server configured.
	client := claudesdk.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx,
		claudesdk.WithLogger(logger),
		claudesdk.WithMCPServers(map[string]claudesdk.MCPServerConfig{
			"calc": calculator,
		}),
	); err != nil {
		logger.Error("Failed to start client", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	// Query MCP server status.
	status, err := client.GetMCPStatus(ctx)
	if err != nil {
		logger.Error("Failed to get MCP status", "error", err)
		os.Exit(1)
	}

	fmt.Println("MCP Server Status:")

	for _, server := range status.MCPServers {
		fmt.Printf("  %s: %s\n", server.Name, server.Status)
	}
}
