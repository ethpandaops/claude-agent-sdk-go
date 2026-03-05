package main

import (
	"context"
	"fmt"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

func onUserInput(
	_ context.Context,
	req *claudesdk.UserInputRequest,
) (*claudesdk.UserInputResponse, error) {
	answers := make(map[string]*claudesdk.UserInputAnswer, len(req.Questions))

	for _, question := range req.Questions {
		fmt.Printf("Question: %s\n", question.Question)

		if len(question.Options) > 0 {
			fmt.Println("Options:")

			for _, option := range question.Options {
				if option.Description != "" {
					fmt.Printf("  - %s: %s\n", option.Label, option.Description)
				} else {
					fmt.Printf("  - %s\n", option.Label)
				}
			}

			fmt.Printf("Auto-selecting first option: %s\n", question.Options[0].Label)
			answers[question.Question] = &claudesdk.UserInputAnswer{
				Answers: []string{question.Options[0].Label},
			}

			continue
		}

		answers[question.Question] = &claudesdk.UserInputAnswer{
			Answers: []string{"Automated response"},
		}
	}

	return &claudesdk.UserInputResponse{Answers: answers}, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := claudesdk.NewClient()
	defer client.Close()

	err := client.Start(ctx,
		claudesdk.WithPermissionMode("plan"),
		claudesdk.WithOnUserInput(onUserInput),
	)
	if err != nil {
		fmt.Printf("Start failed: %v\n", err)

		return
	}

	err = client.Query(ctx,
		"Use AskUserQuestion to ask me to choose between Go, Rust, and Python. "+
			"After I answer, confirm the selected language in one sentence.",
	)
	if err != nil {
		fmt.Printf("Query failed: %v\n", err)

		return
	}

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			fmt.Printf("Receive error: %v\n", err)

			return
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			for _, block := range m.Content {
				switch b := block.(type) {
				case *claudesdk.TextBlock:
					fmt.Printf("Assistant: %s\n", b.Text)
				case *claudesdk.ToolUseBlock:
					fmt.Printf("ToolUse: %s\n", b.Name)
				}
			}

		case *claudesdk.ResultMessage:
			fmt.Printf("Done. is_error=%v\n", m.IsError)

			return
		}
	}
}
