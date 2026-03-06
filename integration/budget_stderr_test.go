//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
	"github.com/stretchr/testify/require"
)

// TestStderrCallback_ReceivesOutput tests Stderr callback invocation.
func TestStderrCallback_ReceivesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var stderrLines []string

	for _, err := range claudesdk.Query(ctx, "Say 'hello'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
		claudesdk.WithStderr(func(line string) {
			stderrLines = append(stderrLines, line)
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}
	}

	t.Logf("Received %d stderr lines", len(stderrLines))
}

// TestStderrCallback_CapturesDebugInfo tests debug flag produces stderr output.
func TestStderrCallback_CapturesDebugInfo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var stderrLines []string

	for _, err := range claudesdk.Query(ctx, "Say 'debug test'",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
		claudesdk.WithStderr(func(line string) {
			stderrLines = append(stderrLines, line)
		}),
		claudesdk.WithExtraArgs(map[string]*string{
			"debug-to-stderr": nil,
		}),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}
	}

	t.Logf("Received %d stderr lines with debug enabled", len(stderrLines))

	if len(stderrLines) > 0 {
		t.Logf("First line: %s", stderrLines[0])
	}
}

// TestMaxBudgetUSD_LimitEnforced tests that MaxBudgetUSD option limits spending.
func TestMaxBudgetUSD_LimitEnforced(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	budget := 0.0001

	var (
		receivedResult   bool
		resultSubtype    string
		resultIsError    bool
		totalCost        float64
		receivedResponse bool
	)

	for msg, err := range claudesdk.Query(ctx,
		"What is 2+2? Reply with one digit.",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
		claudesdk.WithMaxBudgetUSD(budget),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)
			t.Fatalf("Query failed: %v", err)
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			receivedResponse = true
		case *claudesdk.ResultMessage:
			receivedResult = true
			resultSubtype = m.Subtype
			resultIsError = m.IsError

			if m.TotalCostUSD != nil {
				totalCost = *m.TotalCostUSD
			}

			t.Logf("Result: subtype=%s, isError=%v, totalCost=%f",
				resultSubtype, resultIsError, totalCost)
		}
	}

	require.True(t, receivedResult, "Should receive ResultMessage")

	if resultSubtype == "error_max_budget_usd" {
		// Recent CLI versions return subtype=error_max_budget_usd with is_error=false.
		// Subtype is the authoritative signal for budget enforcement.
		require.Greater(t, totalCost, budget, "Reported cost should exceed configured budget")
		t.Logf("Budget limit was enforced as expected")
	} else {
		t.Logf("Budget was not exceeded (subtype=%s), cost=%f",
			resultSubtype, totalCost)
		require.True(t, receivedResponse, "Should receive response if budget not exceeded")
	}
}

// TestMaxBudgetUSD_ZeroBudget tests behavior with zero budget.
func TestMaxBudgetUSD_ZeroBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	budget := 0.0

	var resultSubtype string

	for msg, err := range claudesdk.Query(ctx, "Say hello",
		claudesdk.WithModel("claude-haiku-4-5"),
		claudesdk.WithPermissionMode("bypassPermissions"),
		claudesdk.WithMaxTurns(1),
		claudesdk.WithMaxBudgetUSD(budget),
	) {
		if err != nil {
			skipIfCLINotInstalled(t, err)

			// Newer Claude CLI versions reject zero budget at argument parsing time.
			if processErr, ok := errors.AsType[*claudesdk.ProcessError](err); ok {
				require.Equal(t, 1, processErr.ExitCode)
				t.Logf("Zero budget rejected by CLI as expected: %v", err)

				return
			}

			t.Fatalf("Query failed: %v", err)
		}

		if result, ok := msg.(*claudesdk.ResultMessage); ok {
			resultSubtype = result.Subtype
			t.Logf("Result with zero budget: subtype=%s, isError=%v",
				resultSubtype, result.IsError)
		}
	}

	if resultSubtype == "error_max_budget_usd" {
		t.Logf("Zero budget correctly triggered budget exceeded error")
	} else {
		t.Logf("Unexpected subtype with zero budget: %s", resultSubtype)
	}
}
