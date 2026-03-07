package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

const structuredOutputModel = "claude-haiku-4-5"

// Person represents a simple structured output schema.
type Person struct {
	Name    string   `json:"name"`
	Age     int      `json:"age"`
	Hobbies []string `json:"hobbies"`
}

// BookReview represents a more complex nested structured output.
type BookReview struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Rating int    `json:"rating"`
	Review struct {
		Summary  string   `json:"summary"`
		Pros     []string `json:"pros"`
		Cons     []string `json:"cons"`
		Audience string   `json:"audience"`
	} `json:"review"`
}

// getStructuredOutput runs a query and returns the structured output from
// the ResultMessage. When WithOutputFormat() is used, the CLI validates
// the response against the schema and provides the result as JSON.
// It checks StructuredOutput first (parsed object), then falls back to
// the Result field (JSON string).
func getStructuredOutput(
	ctx context.Context,
	msgs func(func(claudesdk.Message, error) bool),
) (json.RawMessage, error) {
	for msg, err := range msgs {
		if err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}

		if m, ok := msg.(*claudesdk.ResultMessage); ok {
			if m.TotalCostUSD != nil {
				fmt.Printf("Cost: $%.4f\n", *m.TotalCostUSD)
			}

			// Prefer StructuredOutput (pre-parsed object).
			if m.StructuredOutput != nil {
				data, err := json.Marshal(m.StructuredOutput)
				if err != nil {
					return nil, fmt.Errorf("marshal structured output: %w", err)
				}

				return data, nil
			}

			// Fall back to Result field (JSON string from --json-schema).
			if m.Result != nil && json.Valid([]byte(*m.Result)) {
				return json.RawMessage(*m.Result), nil
			}
		}
	}

	return nil, fmt.Errorf("no structured output received")
}

func simpleStructuredOutput() {
	fmt.Println("=== Simple Structured Output ===")
	fmt.Println("Using WithOutputFormat() to get a JSON Person object.")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// WithOutputFormat accepts either the canonical wrapped format:
	//   {"type": "json_schema", "schema": <json schema>}
	// or a raw JSON schema that the SDK auto-detects and uses directly:
	//   {"type": "object", "properties": {...}}
	// The SDK passes the schema via --json-schema to the CLI, which constrains
	// the model to output valid JSON matching the schema.
	outputFormat := map[string]any{
		"type": "json_schema",
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"name":    map[string]any{"type": "string"},
				"age":     map[string]any{"type": "integer"},
				"hobbies": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
			"required": []string{"name", "age", "hobbies"},
		},
	}

	output, err := getStructuredOutput(ctx, claudesdk.Query(ctx,
		"Invent a fictional person with a name, age, and exactly 3 hobbies.",
		claudesdk.WithModel(structuredOutputModel),
		claudesdk.WithOutputFormat(outputFormat),
		claudesdk.WithSystemPrompt("You are a creative writer."),
		claudesdk.WithMaxTurns(2),
	))
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	var person Person
	if err := json.Unmarshal(output, &person); err != nil {
		fmt.Printf("Failed to parse JSON: %v\n", err)
		fmt.Printf("Raw output: %s\n", string(output))

		return
	}

	fmt.Printf("Name:    %s\n", person.Name)
	fmt.Printf("Age:     %d\n", person.Age)
	fmt.Printf("Hobbies: %v\n", person.Hobbies)
	fmt.Println()
}

func nestedStructuredOutput() {
	fmt.Println("=== Nested Structured Output ===")
	fmt.Println("Using WithOutputFormat() with a complex nested schema.")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	outputFormat := map[string]any{
		"type": "json_schema",
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"title":  map[string]any{"type": "string"},
				"author": map[string]any{"type": "string"},
				"rating": map[string]any{"type": "integer", "minimum": 1, "maximum": 5},
				"review": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"summary":  map[string]any{"type": "string"},
						"pros":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"cons":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"audience": map[string]any{"type": "string"},
					},
					"required": []string{"summary", "pros", "cons", "audience"},
				},
			},
			"required": []string{"title", "author", "rating", "review"},
		},
	}

	output, err := getStructuredOutput(ctx, claudesdk.Query(ctx,
		"Write a brief review of '1984' by George Orwell. "+
			"Include title, author, a rating from 1-5, and a review with "+
			"a short summary, 2 pros, 2 cons, and target audience.",
		claudesdk.WithModel(structuredOutputModel),
		claudesdk.WithOutputFormat(outputFormat),
		claudesdk.WithSystemPrompt("You are a book critic."),
		claudesdk.WithMaxTurns(2),
	))
	if err != nil {
		fmt.Printf("Error: %v\n", err)

		return
	}

	var review BookReview
	if err := json.Unmarshal(output, &review); err != nil {
		fmt.Printf("Failed to parse JSON: %v\n", err)
		fmt.Printf("Raw output: %s\n", string(output))

		return
	}

	fmt.Printf("Title:    %s\n", review.Title)
	fmt.Printf("Author:   %s\n", review.Author)
	fmt.Printf("Rating:   %d/5\n", review.Rating)
	fmt.Printf("Summary:  %s\n", review.Review.Summary)
	fmt.Printf("Pros:     %v\n", review.Review.Pros)
	fmt.Printf("Cons:     %v\n", review.Review.Cons)
	fmt.Printf("Audience: %s\n", review.Review.Audience)
	fmt.Println()
}

func main() {
	fmt.Println("Structured Output Examples")
	fmt.Println()
	fmt.Println("Demonstrates using WithOutputFormat() to get structured JSON responses.")
	fmt.Println()

	simpleStructuredOutput()
	nestedStructuredOutput()
}
