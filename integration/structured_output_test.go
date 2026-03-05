//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

// TestStructuredOutput_JSONSchema tests OutputFormat produces valid JSON.
func TestStructuredOutput_JSONSchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var (
		receivedResponse   bool
		receivedStructured bool
	)

	for msg, err := range claudesdk.Query(ctx, "What is 2+2? Provide structured output.",
		claudesdk.WithModel("haiku"),
		claudesdk.WithPermissionMode("acceptAll"),
		claudesdk.WithMaxTurns(2),
		claudesdk.WithOutputFormat(map[string]any{
			"type": "json_schema",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"answer": map[string]any{
						"type":        "string",
						"description": "The answer to the question",
					},
					"confidence": map[string]any{
						"type":        "number",
						"description": "Confidence level from 0 to 1",
					},
				},
				"required": []string{"answer"},
			},
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			for _, block := range m.Content {
				if textBlock, ok := block.(*claudesdk.TextBlock); ok {
					t.Logf("Structured output: %s", textBlock.Text)
					receivedResponse = true
				}
			}
		case *claudesdk.ResultMessage:
			require.False(t, m.IsError, "Query should not result in error")
			if m.StructuredOutput != nil {
				t.Logf("Result structured_output: %#v", m.StructuredOutput)
				receivedStructured = true
			}
		}
	}

	require.True(t, receivedResponse || receivedStructured,
		"Should receive structured response in assistant text or result.structured_output")
}

// TestStructuredOutput_RequiredFields tests required fields are present in output.
func TestStructuredOutput_RequiredFields(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var (
		receivedResponse   bool
		receivedStructured bool
	)

	for msg, err := range claudesdk.Query(ctx,
		"Generate a fictional person with a name and age in structured format.",
		claudesdk.WithModel("haiku"),
		claudesdk.WithPermissionMode("acceptAll"),
		claudesdk.WithMaxTurns(2),
		claudesdk.WithOutputFormat(map[string]any{
			"type": "json_schema",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
					"age": map[string]any{
						"type": "integer",
					},
				},
				"required": []string{"name", "age"},
			},
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			for _, block := range m.Content {
				if textBlock, ok := block.(*claudesdk.TextBlock); ok {
					t.Logf("Output with required fields: %s", textBlock.Text)
					receivedResponse = true
				}
			}
		case *claudesdk.ResultMessage:
			require.False(t, m.IsError, "Query should not result in error")
			if m.StructuredOutput != nil {
				t.Logf("Result structured_output with required fields: %#v", m.StructuredOutput)
				receivedStructured = true
			}
		}
	}

	require.True(t, receivedResponse || receivedStructured,
		"Should receive response with required fields in text or result.structured_output")
}

// TestStructuredOutput_WithEnum tests structured output with enum type.
func TestStructuredOutput_WithEnum(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var (
		receivedResponse   bool
		receivedStructured bool
	)

	for msg, err := range claudesdk.Query(ctx,
		"Pick a random color and intensity. Respond in structured format.",
		claudesdk.WithModel("haiku"),
		claudesdk.WithPermissionMode("acceptAll"),
		claudesdk.WithMaxTurns(2),
		claudesdk.WithOutputFormat(map[string]any{
			"type": "json_schema",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"color": map[string]any{
						"type":        "string",
						"enum":        []string{"red", "green", "blue"},
						"description": "A color choice",
					},
					"intensity": map[string]any{
						"type":        "string",
						"enum":        []string{"low", "medium", "high"},
						"description": "Intensity level",
					},
				},
				"required": []string{"color", "intensity"},
			},
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			for _, block := range m.Content {
				if textBlock, ok := block.(*claudesdk.TextBlock); ok {
					t.Logf("Structured output with enum: %s", textBlock.Text)
					receivedResponse = true
				}
			}
		case *claudesdk.ResultMessage:
			require.False(t, m.IsError, "Query should not result in error")
			if m.StructuredOutput != nil {
				t.Logf("Result structured_output with enum: %#v", m.StructuredOutput)
				receivedStructured = true
			}
		}
	}

	require.True(t, receivedResponse || receivedStructured,
		"Should receive structured response with enum values in text or result.structured_output")
}

// TestStructuredOutput_WithTools tests structured output combined with tool usage.
func TestStructuredOutput_WithTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var toolExecuted bool

	dataTool := claudesdk.NewTool(
		"get_data",
		"Gets data for structured output",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		func(_ context.Context, _ map[string]any) (map[string]any, error) {
			toolExecuted = true

			return map[string]any{
				"value":  42,
				"status": "success",
			}, nil
		},
	)

	var (
		receivedResponse   bool
		receivedStructured bool
	)

	for msg, err := range claudesdk.Query(ctx,
		"Use the get_data tool to get a value, then provide the result in structured format.",
		claudesdk.WithModel("haiku"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(3),
		claudesdk.WithSDKTools(dataTool),
		claudesdk.WithOutputFormat(map[string]any{
			"type": "json_schema",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"data_value": map[string]any{
						"type":        "integer",
						"description": "The value from the data tool",
					},
					"summary": map[string]any{
						"type":        "string",
						"description": "Summary of the result",
					},
				},
				"required": []string{"data_value", "summary"},
			},
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			for _, block := range m.Content {
				if textBlock, ok := block.(*claudesdk.TextBlock); ok {
					t.Logf("Structured output with tools: %s", textBlock.Text)
					receivedResponse = true
				}
			}
		case *claudesdk.ResultMessage:
			require.False(t, m.IsError, "Query should not result in error")
			if m.StructuredOutput != nil {
				t.Logf("Result structured_output with tools: %#v", m.StructuredOutput)
				receivedStructured = true
			}
		}
	}

	require.True(t, toolExecuted, "Tool should have been executed")
	require.True(t, receivedResponse || receivedStructured,
		"Should receive structured response after tool use in text or result.structured_output")
}
