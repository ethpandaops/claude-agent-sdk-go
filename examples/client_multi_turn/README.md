# Streaming Client Examples

Comprehensive examples demonstrating various patterns for building applications with the Claude SDK Go streaming interface.

## Overview

This example collection showcases 10 different patterns for interactive client usage:

1. **basic_streaming** - Basic streaming with context manager
2. **multi_turn_conversation** - Multi-turn conversations
3. **concurrent_responses** - Concurrent send/receive using goroutines
4. **with_interrupt** - Interrupt capability demonstration
5. **manual_message_handling** - Manual message stream handling with custom logic
6. **with_options** - Using functional options for configuration
7. **async_iterable_prompt** - Channel-based prompt streaming
8. **bash_command** - Tool use blocks when running bash commands
9. **control_protocol** - Control protocol capabilities (SetPermissionMode, SetModel)
10. **error_handling** - Error handling patterns for API errors

## Usage

Run a specific example:
```bash
go run main.go <example_name>
```

Run all examples sequentially:
```bash
go run main.go all
```

List available examples:
```bash
go run main.go
```

## Examples

### Basic Streaming
Simple query and response pattern using the helper method `ReceiveResponse()`.

```bash
go run main.go basic_streaming
```

### Multi-Turn Conversation
Demonstrates maintaining context across multiple conversation turns.

```bash
go run main.go multi_turn_conversation
```

### Concurrent Send/Receive
Shows how to handle responses while sending new messages using goroutines and channels.

```bash
go run main.go concurrent_responses
```

### Interrupt
Demonstrates how to interrupt a long-running task and send a new query.

```bash
go run main.go with_interrupt
```

### Manual Message Handling
Process messages manually with custom logic - extracts programming language names from responses.

```bash
go run main.go manual_message_handling
```

### Custom Options
Configure the client with functional options including allowed tools, system prompts, and permission modes.

```bash
go run main.go with_options
```

### Async Iterable Prompt
Stream messages to Claude using a channel-based iterator pattern.

```bash
go run main.go async_iterable_prompt
```

### Bash Command
Shows tool use blocks when Claude executes bash commands.

```bash
go run main.go bash_command
```

### Control Protocol
Demonstrates runtime control capabilities like changing permission modes and models.

```bash
go run main.go control_protocol
```

### Error Handling
Demonstrates checking `AssistantMessage.Error` for API-level errors in responses.

```bash
go run main.go error_handling
```

## Key Patterns

### Context Management
All examples use `context.WithTimeout` for proper lifecycle management:

```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
```

### Client Lifecycle
```go
client := claudesdk.NewClient()
defer client.Close()

if err := client.Start(ctx,
    claudesdk.WithPermissionMode("acceptEdits"),
); err != nil {
    // handle error
}
```

### Message Processing
```go
// Iterator-based response (stops at ResultMessage)
for msg, err := range client.ReceiveResponse(ctx) {
    if err != nil {
        // handle error
        break
    }
    // process message
}

// Continuous message streaming (yields indefinitely)
for msg, err := range client.ReceiveMessages(ctx) {
    if err != nil {
        break
    }
    // process message
    if _, ok := msg.(*claudesdk.ResultMessage); ok {
        break // exit when done
    }
}
```

### Concurrent Operations
```go
var wg sync.WaitGroup
done := make(chan struct{})

wg.Add(1)
go func() {
    defer wg.Done()
    // Use iter.Pull2 for pull-based iteration in goroutines with select
    next, stop := iter.Pull2(client.ReceiveMessages(ctx))
    defer stop()
    for {
        select {
        case <-done:
            return
        default:
            msg, err, ok := next()
            if !ok || err != nil {
                return
            }
            // process message
        }
    }
}()

// ... do work ...
close(done)
wg.Wait()
```

## Notes

The queries in these examples are intentionally simplistic. In real applications, queries can be complex tasks where Claude SDK uses its agentic capabilities and tools (bash commands, file operations, web search, etc.) to accomplish goals.
