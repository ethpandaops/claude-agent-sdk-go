// Package main demonstrates how to create calculator tools using MCP servers.
//
// This example shows how to create an in-process MCP server with calculator
// tools using the Claude SDK with the official MCP SDK types.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// createCalculatorTools creates the 6 calculator tools: add, subtract, multiply, divide, sqrt, power.
func createCalculatorTools() []*claudesdk.SdkMcpTool {
	// Annotations shared by all calculator tools: read-only and idempotent.
	calcAnnotations := &mcp.ToolAnnotations{
		ReadOnlyHint:   true,
		IdempotentHint: true,
	}

	// Add tool - using simple type schema
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
			result := a + b

			return claudesdk.TextResult(fmt.Sprintf("%v + %v = %v", a, b, result)), nil
		},
		claudesdk.WithAnnotations(calcAnnotations),
	)

	// Subtract tool
	subtractTool := claudesdk.NewSdkMcpTool(
		"subtract",
		"Subtract one number from another",
		claudesdk.SimpleSchema(map[string]string{"a": "float64", "b": "float64"}),
		func(_ context.Context, req *claudesdk.CallToolRequest) (*claudesdk.CallToolResult, error) {
			args, err := claudesdk.ParseArguments(req)
			if err != nil {
				return claudesdk.ErrorResult(err.Error()), nil
			}

			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			result := a - b

			return claudesdk.TextResult(fmt.Sprintf("%v - %v = %v", a, b, result)), nil
		},
		claudesdk.WithAnnotations(calcAnnotations),
	)

	// Multiply tool
	multiplyTool := claudesdk.NewSdkMcpTool(
		"multiply",
		"Multiply two numbers",
		claudesdk.SimpleSchema(map[string]string{"a": "float64", "b": "float64"}),
		func(_ context.Context, req *claudesdk.CallToolRequest) (*claudesdk.CallToolResult, error) {
			args, err := claudesdk.ParseArguments(req)
			if err != nil {
				return claudesdk.ErrorResult(err.Error()), nil
			}

			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			result := a * b

			return claudesdk.TextResult(fmt.Sprintf("%v × %v = %v", a, b, result)), nil
		},
		claudesdk.WithAnnotations(calcAnnotations),
	)

	// Divide tool
	divideTool := claudesdk.NewSdkMcpTool(
		"divide",
		"Divide one number by another",
		claudesdk.SimpleSchema(map[string]string{"a": "float64", "b": "float64"}),
		func(_ context.Context, req *claudesdk.CallToolRequest) (*claudesdk.CallToolResult, error) {
			args, err := claudesdk.ParseArguments(req)
			if err != nil {
				return claudesdk.ErrorResult(err.Error()), nil
			}

			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)

			if b == 0 {
				return claudesdk.ErrorResult("Error: Division by zero is not allowed"), nil
			}

			result := a / b

			return claudesdk.TextResult(fmt.Sprintf("%v ÷ %v = %v", a, b, result)), nil
		},
		claudesdk.WithAnnotations(calcAnnotations),
	)

	// Square root tool
	sqrtTool := claudesdk.NewSdkMcpTool(
		"sqrt",
		"Calculate square root",
		claudesdk.SimpleSchema(map[string]string{"n": "float64"}),
		func(_ context.Context, req *claudesdk.CallToolRequest) (*claudesdk.CallToolResult, error) {
			args, err := claudesdk.ParseArguments(req)
			if err != nil {
				return claudesdk.ErrorResult(err.Error()), nil
			}

			n, _ := args["n"].(float64)

			if n < 0 {
				return claudesdk.ErrorResult(
					fmt.Sprintf("Error: Cannot calculate square root of negative number %v", n),
				), nil
			}

			result := math.Sqrt(n)

			return claudesdk.TextResult(fmt.Sprintf("√%v = %v", n, result)), nil
		},
		claudesdk.WithAnnotations(calcAnnotations),
	)

	// Power tool
	powerTool := claudesdk.NewSdkMcpTool(
		"power",
		"Raise a number to a power",
		claudesdk.SimpleSchema(map[string]string{"base": "float64", "exponent": "float64"}),
		func(_ context.Context, req *claudesdk.CallToolRequest) (*claudesdk.CallToolResult, error) {
			args, err := claudesdk.ParseArguments(req)
			if err != nil {
				return claudesdk.ErrorResult(err.Error()), nil
			}

			base, _ := args["base"].(float64)
			exponent, _ := args["exponent"].(float64)
			result := math.Pow(base, exponent)

			return claudesdk.TextResult(fmt.Sprintf("%v^%v = %v", base, exponent, result)), nil
		},
		claudesdk.WithAnnotations(calcAnnotations),
	)

	return []*claudesdk.SdkMcpTool{addTool, subtractTool, multiplyTool, divideTool, sqrtTool, powerTool}
}

// displayMessage displays message content in a clean format.
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
			switch b := block.(type) {
			case *claudesdk.TextBlock:
				fmt.Printf("Claude: %s\n", b.Text)
			case *claudesdk.ToolUseBlock:
				fmt.Printf("Using tool: %s\n", b.Name)

				if len(b.Input) > 0 {
					fmt.Printf("  Input: ")

					first := true

					for k, v := range b.Input {
						if !first {
							fmt.Print(", ")
						}

						fmt.Printf("%s=%v", k, v)

						first = false
					}

					fmt.Println()
				}
			}
		}

	case *claudesdk.SystemMessage:
		// Ignore system messages

	case *claudesdk.ResultMessage:
		fmt.Println("Result ended")

		if m.TotalCostUSD != nil {
			fmt.Printf("Cost: $%.6f\n", *m.TotalCostUSD)
		}
	}
}

func main() {
	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create calculator tools
	tools := createCalculatorTools()

	// Create the calculator MCP server config
	// The name "calc" is used as the server key for tool naming (mcp__calc__<toolName>)
	calculator := claudesdk.CreateSdkMcpServer("calc", "2.0.0", tools...)

	// Example prompts to demonstrate calculator usage
	prompts := []string{
		"List your tools",
		"Calculate 15 + 27",
		"What is 100 divided by 7?",
		"Calculate the square root of 144",
		"What is 2 raised to the power of 8?",
		"Calculate (12 + 8) * 3 - 10",
	}

	for _, prompt := range prompts {
		fmt.Printf("\n%s\n", "==================================================")
		fmt.Printf("Prompt: %s\n", prompt)
		fmt.Printf("%s\n", "==================================================")

		client := claudesdk.NewClient()

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

		if err := client.Start(ctx,
			claudesdk.WithLogger(logger),
			claudesdk.WithMCPServers(map[string]claudesdk.MCPServerConfig{
				"calc": calculator,
			}),
			claudesdk.WithAllowedTools(
				"mcp__calc__add",
				"mcp__calc__subtract",
				"mcp__calc__multiply",
				"mcp__calc__divide",
				"mcp__calc__sqrt",
				"mcp__calc__power",
			),
			claudesdk.WithMaxTurns(10),
		); err != nil {
			logger.Error("Failed to connect", "error", err)
			cancel()
			client.Close()
			os.Exit(1)
		}

		if err := client.Query(ctx, prompt); err != nil {
			logger.Error("Failed to send query", "error", err)
			cancel()
			client.Close()
			os.Exit(1)
		}

		for msg, err := range client.ReceiveResponse(ctx) {
			if err != nil {
				logger.Error("Failed to receive response", "error", err)
				cancel()
				client.Close()
				os.Exit(1)
			}

			displayMessage(msg)
		}

		cancel()
		client.Close()
	}
}
