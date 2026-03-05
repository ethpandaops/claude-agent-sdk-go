// Package mcp implements an in-process Model Context Protocol server.
//
// The MCP server allows users to register custom tools that Claude can invoke
// during execution. Tools are registered via the SDK and exposed through the
// protocol controller's MCP message handler.
//
// The server maintains a thread-safe registry of tools and handles tool
// listing and execution requests from the Claude CLI.
package mcp
