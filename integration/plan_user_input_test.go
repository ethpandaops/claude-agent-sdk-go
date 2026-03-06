//go:build integration

package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
	"github.com/stretchr/testify/require"
)

func TestPlanMode_UserInputCallback_Client(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "5")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var callbackInvoked atomic.Bool

	onUserInput := func(
		_ context.Context,
		req *claudesdk.UserInputRequest,
	) (*claudesdk.UserInputResponse, error) {
		callbackInvoked.Store(true)

		answers := make(map[string]*claudesdk.UserInputAnswer, len(req.Questions))
		for _, q := range req.Questions {
			if len(q.Options) > 0 {
				answers[q.Question] = &claudesdk.UserInputAnswer{
					Answers: []string{q.Options[0].Label},
				}
			} else {
				answers[q.Question] = &claudesdk.UserInputAnswer{
					Answers: []string{"Go"},
				}
			}
		}

		return &claudesdk.UserInputResponse{Answers: answers}, nil
	}

	client := claudesdk.NewClient()
	defer client.Close()

	err := client.Start(ctx,
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("plan"),
		claudesdk.WithOnUserInput(onUserInput),
		claudesdk.WithMaxTurns(5),
	)
	if err != nil {
		skipIfCLINotInstalled(t, err)
		t.Fatalf("Start failed: %v", err)
	}

	err = client.Query(ctx,
		"Use AskUserQuestion to ask me to choose Go or Rust. "+
			"After I answer, confirm my selected language in one sentence.",
	)
	require.NoError(t, err)

	var gotResult bool

	for msg, err := range client.ReceiveMessages(ctx) {
		if err != nil {
			t.Fatalf("ReceiveMessages failed: %v", err)
		}

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			gotResult = true
			require.False(t, result.IsError)

			break
		}
	}

	require.True(t, callbackInvoked.Load(), "OnUserInput callback should be invoked")
	require.True(t, gotResult, "Expected a ResultMessage")
}

func TestPlanMode_UserInputCallback_Query(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "5")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var callbackInvoked atomic.Bool

	onUserInput := func(
		_ context.Context,
		req *claudesdk.UserInputRequest,
	) (*claudesdk.UserInputResponse, error) {
		callbackInvoked.Store(true)

		answers := make(map[string]*claudesdk.UserInputAnswer, len(req.Questions))
		for _, q := range req.Questions {
			answers[q.Question] = &claudesdk.UserInputAnswer{
				Answers: []string{"Go"},
			}
		}

		return &claudesdk.UserInputResponse{Answers: answers}, nil
	}

	var gotResult bool

	for msg, err := range claudesdk.Query(ctx,
		"Use AskUserQuestion to ask me to choose Go or Rust. "+
			"After I answer, confirm my selected language in one sentence.",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("plan"),
		claudesdk.WithOnUserInput(onUserInput),
		claudesdk.WithMaxTurns(5),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			gotResult = true
			require.False(t, result.IsError)
		}
	}

	require.True(t, callbackInvoked.Load(), "OnUserInput callback should be invoked")
	require.True(t, gotResult, "Expected a ResultMessage")
}
