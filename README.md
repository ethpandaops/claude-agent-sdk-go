# Claude Agent SDK Go

Go SDK for building agentic applications with Claude Code.

## Installation

```bash
go get github.com/ethpandaops/claude-agent-sdk-go
```

**Prerequisites:**
- Go 1.26+
- Claude Code CLI v2.1.59+ (`npm install -g @anthropic-ai/claude-code`)

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    claudesdk "github.com/ethpandaops/claude-agent-sdk-go"
)

func main() {
    ctx := context.Background()
    for msg, err := range claudesdk.Query(ctx, "What is 2 + 2?") {
        if err != nil {
            panic(err)
        }
        if result, ok := msg.(claudesdk.ResultMessage); ok {
            fmt.Println(result.Result)
        }
    }
}
```

## Basic Usage: Query()

One-shot query execution returning an iterator of messages.

```go
for msg, err := range claudesdk.Query(ctx, "Explain Go interfaces") {
    // handle msg
}
```

### With Options

```go
for msg, err := range claudesdk.Query(ctx, "Hello",
    claudesdk.WithSystemPrompt("You are a helpful assistant"),
    claudesdk.WithModel("claude-sonnet-4-20250514"),
    claudesdk.WithMaxTurns(3),
) {
    // handle msg
}
```

### With Tools

```go
for msg, err := range claudesdk.Query(ctx, "Create hello.py that prints hello world",
    claudesdk.WithAllowedTools("Read", "Write"),
    claudesdk.WithPermissionMode("acceptEdits"),
) {
    // handle msg
}
```

## Client (Multi-turn)

`WithClient` manages the client lifecycle for multi-turn conversations.

```go
err := claudesdk.WithClient(ctx, func(c claudesdk.Client) error {
    if err := c.Query(ctx, "Hello, remember my name is Alice"); err != nil {
        return err
    }
    for msg, err := range c.ReceiveResponse(ctx) {
        if err != nil {
            return err
        }
        // handle response
    }

    if err := c.Query(ctx, "What's my name?"); err != nil {
        return err
    }
    for msg, err := range c.ReceiveResponse(ctx) {
        // Claude remembers: "Alice"
    }
    return nil
})
```

## SDK MCP Servers (Custom Tools)

Create in-process tools using the Model Context Protocol.

```go
schema := claudesdk.SimpleSchema(map[string]string{"a": "number", "b": "number"})

handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args, _ := claudesdk.ParseArguments(req)
    sum := args["a"].(float64) + args["b"].(float64)
    return claudesdk.TextResult(fmt.Sprintf("%.0f", sum)), nil
}

tool := claudesdk.NewSdkMcpTool("add", "Add two numbers", schema, handler)
server := claudesdk.CreateSdkMcpServer("calc", "1.0.0", tool)

for msg, err := range claudesdk.Query(ctx, "Calculate 2 + 2",
    claudesdk.WithMCPServers(map[string]claudesdk.MCPServerConfig{"calc": server}),
    claudesdk.WithAllowedTools("mcp__calc__add"),
) {
    // handle msg
}
```

## Hooks

Intercept and modify tool execution.

```go
handler := func(ctx context.Context, hookCtx claudesdk.HookContext, input claudesdk.HookInput) (claudesdk.HookJSONOutput, error) {
    pre := input.(*claudesdk.PreToolUseHookInput)
    fmt.Printf("Tool: %s\n", pre.ToolName)
    return &claudesdk.SyncHookJSONOutput{}, nil
}

for msg, err := range claudesdk.Query(ctx, prompt,
    claudesdk.WithHooks(map[claudesdk.HookEvent][]*claudesdk.HookMatcher{
        claudesdk.HookEventPreToolUse: {{
            ToolName: "Bash",
            Hooks:    []claudesdk.HookCallback{handler},
        }},
    }),
) {
    // handle msg
}
```

## Plan Mode User Input

Handle `AskUserQuestion` prompts with a typed callback:

```go
onUserInput := func(
    _ context.Context,
    req *claudesdk.UserInputRequest,
) (*claudesdk.UserInputResponse, error) {
    answers := map[string]*claudesdk.UserInputAnswer{}
    for _, q := range req.Questions {
        if len(q.Options) > 0 {
            answers[q.Question] = &claudesdk.UserInputAnswer{
                Answers: []string{q.Options[0].Label},
            }
        }
    }
    return &claudesdk.UserInputResponse{Answers: answers}, nil
}

for msg, err := range claudesdk.Query(ctx,
    "Use AskUserQuestion to ask me to pick Go or Rust, then confirm my choice.",
    claudesdk.WithPermissionMode("plan"),
    claudesdk.WithOnUserInput(onUserInput),
) {
    _ = msg
    _ = err
}
```

## Types

Core message types implement the `Message` interface:

- `UserMessage` - User input
- `AssistantMessage` - Claude response with `Content []ContentBlock`
- `ResultMessage` - Final result with `Result string`
- `SystemMessage` - System messages

Content blocks: `TextBlock`, `ThinkingBlock`, `ToolUseBlock`, `ToolResultBlock`

See [types.go](./types.go) for complete type definitions.

## Error Handling

SDK errors can be inspected using `errors.AsType` (Go 1.26+):

```go
if cliErr, ok := errors.AsType[*claudesdk.CLINotFoundError](err); ok {
    fmt.Println("Claude CLI not found:", cliErr)
}

if procErr, ok := errors.AsType[*claudesdk.ProcessError](err); ok {
    fmt.Println("Process failed:", procErr)
}
```

Error types:
- `CLINotFoundError` - Claude CLI binary not found
- `CLIConnectionError` - Failed to connect to CLI
- `ProcessError` - CLI process failure
- `MessageParseError` - Message parsing failure
- `CLIJSONDecodeError` - JSON decode failure

Sentinel errors: `ErrClientNotConnected`, `ErrClientAlreadyConnected`, `ErrClientClosed`, `ErrSessionNotFound`

## Session Metadata

Inspect locally persisted session metadata without sending a prompt:

```go
stat, err := claudesdk.StatSession(ctx, sessionID,
    claudesdk.WithStatProjectPath("/path/to/project"), // optional
    claudesdk.WithStatClaudeHome("/custom/.claude"),   // optional
)
if err != nil {
    if errors.Is(err, claudesdk.ErrSessionNotFound) {
        // session does not exist in local persistence
    }
    // handle other errors
}
fmt.Println(stat.SizeBytes, stat.LastModified)
```

## Examples

See the [examples](./examples) directory for complete working examples.

### Testing Examples

The `scripts/test_examples.sh` script runs all examples and uses Claude CLI to verify their output.

```bash
# Run all examples
./scripts/test_examples.sh

# Run specific examples
./scripts/test_examples.sh -f hooks,sessions,tools_option

# Keep going on failure
./scripts/test_examples.sh -k

# Adjust parallelism and timeout
./scripts/test_examples.sh -n 3 -t 180
```
