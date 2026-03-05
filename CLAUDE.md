# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Overview

**Claude Agent SDK for Go** - Unofficial Go SDK for Claude Code CLI integration.
Provides `Query()` (one-shot) and `Client` (streaming/multi-turn) APIs.

- **Module**: `github.com/ethpandaops/claude-agent-sdk-go`
- **Go Version**: 1.26+
- **CLI Requirement**: Claude CLI v2.1.59+

## Build & Test

```bash
# Build
go build ./...

# Test
go test ./...                     # All tests
go test -race ./...               # With race detection
go test -v -run TestQuery ./...   # Specific test pattern
go test -tags=integration ./...   # Integration tests (requires CLI)

# Lint (respects .golangci.yml)
golangci-lint run

# Run example
go run ./examples/quick_start
```

## Architecture

```
├── query.go              # Query() - one-shot operations
├── client.go             # Client interface definition
├── client_impl.go        # Client implementation
├── with_client.go        # WithClient() context manager
├── options.go            # WithXxx() functional options
├── types.go              # Message, ContentBlock interfaces
├── errors.go             # Typed errors (CLINotFoundError, ProcessError, etc.)
├── transport.go          # Transport interface abstraction
├── internal/
│   ├── subprocess/       # CLI process management, stdin/stdout buffering
│   ├── protocol/         # Bidirectional control protocol
│   ├── client/           # Stateful client implementation
│   ├── message/          # Message types and parsing
│   ├── mcp/              # MCP server support
│   ├── cli/              # CLI discovery and command building
│   ├── config/           # Configuration and presets
│   ├── hook/             # Hook system
│   ├── models/           # Model registry and capabilities
│   ├── permission/       # Permission callbacks
│   ├── sandbox/          # Sandbox configuration
│   └── errors/           # Internal error types
└── examples/             # 27 usage examples
```

**Data Flow**: Query/Client → Transport interface → subprocess.CLITransport → Claude CLI stdout → message parsing → Message types

## Code Conventions

- **Context-first**: All blocking functions take `context.Context` as first parameter
- **Functional options**: Use `WithXxx()` pattern for configuration (see `options.go`)
- **Error wrapping**: Use `fmt.Errorf` with `%w` verb, include context
- **Interfaces**: Message types implement `Message`, content blocks implement `ContentBlock`
- **Internal packages**: Keep implementation details in `internal/`, re-export public API at root
- **Linting**: Follow `.golangci.yml` rules - 38+ linters enabled

## Boundaries

**Always**:
- Run `go test ./...` after making changes
- Run `golangci-lint run` before commits
- Follow existing patterns in similar code
- Use typed errors from `errors.go` for error handling

**Ask First**:
- Adding new public API surface (exported functions/types)
- Changing internal package interfaces
- Adding new external dependencies
- Modifying the transport interface

**Never**:
- Skip error handling or leave errors unchecked
- Commit without running linter
- Add generic packages (utils/, helpers/, common/)
- Store context in structs

## Testing

- Use `testify/require` for assertions
- Table-driven tests for multiple scenarios
- Integration tests use `//go:build integration` tag
- Mock the `Transport` interface for unit tests
- Run with `-race` flag for concurrency testing
