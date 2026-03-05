package main

import (
	"context"
	"fmt"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

func main() {
	fmt.Println("=== Error Handling Example ===")
	fmt.Println("Demonstrates checking AssistantMessage.Error for API errors.")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for msg, err := range claudesdk.Query(ctx, "Hello, Claude!") {
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)

			return
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			// Check for API-level errors on the assistant message.
			// The CLI reports errors like authentication failures and rate
			// limits via the Error field on assistant messages.
			if m.Error != nil {
				switch *m.Error {
				case claudesdk.AssistantMessageErrorAuthFailed:
					fmt.Println("Authentication failed — check your API key.")
				case claudesdk.AssistantMessageErrorRateLimit:
					fmt.Println("Rate limited — retry after a delay.")
				case claudesdk.AssistantMessageErrorBilling:
					fmt.Println("Billing error — check your account.")
				case claudesdk.AssistantMessageErrorInvalidReq:
					fmt.Println("Invalid request — check parameters.")
				case claudesdk.AssistantMessageErrorServer:
					fmt.Println("Server error — retry later.")
				default:
					fmt.Printf("Unknown error: %s\n", *m.Error)
				}

				return
			}

			// Normal response — print text content.
			for _, block := range m.Content {
				if tb, ok := block.(*claudesdk.TextBlock); ok {
					fmt.Printf("Claude: %s\n", tb.Text)
				}
			}

		case *claudesdk.ResultMessage:
			if m.TotalCostUSD != nil {
				fmt.Printf("Cost: $%.4f\n", *m.TotalCostUSD)
			}
		}
	}
}
