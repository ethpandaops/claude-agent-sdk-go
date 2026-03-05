package claudesdk

import (
	internalmcp "github.com/ethpandaops/claude-agent-sdk-go/internal/mcp"
)

// CreateSdkMcpServer creates an in-process MCP server configuration with SdkMcpTool tools.
//
// This function creates an MCP server
// that runs within your application, providing better performance than external MCP servers.
//
// The returned config can be used directly in ClaudeAgentOptions.MCPServers:
//
//	addTool := claudesdk.NewSdkMcpTool("add", "Add two numbers",
//	    claudesdk.SimpleSchema(map[string]string{"a": "float64", "b": "float64"}),
//	    func(ctx context.Context, req *claudesdk.CallToolRequest) (*claudesdk.CallToolResult, error) {
//	        args, _ := claudesdk.ParseArguments(req)
//	        a, b := args["a"].(float64), args["b"].(float64)
//	        return claudesdk.TextResult(fmt.Sprintf("Result: %v", a+b)), nil
//	    },
//	)
//
//	calculator := claudesdk.CreateSdkMcpServer("calculator", "1.0.0", addTool)
//
//	options := &claudesdk.ClaudeAgentOptions{
//	    MCPServers: map[string]claudesdk.MCPServerConfig{
//	        "calculator": calculator,
//	    },
//	    AllowedTools: []string{"mcp__calculator__add"},
//	}
//
// Parameters:
//   - name: Server name (also used as key in MCPServers map, determines tool naming: mcp__<name>__<toolName>)
//   - version: Server version string
//   - tools: SdkMcpTool instances to register with the server
func CreateSdkMcpServer(name, version string, tools ...*SdkMcpTool) *MCPSdkServerConfig {
	server := internalmcp.NewSDKServer(name, version)

	for _, tool := range tools {
		mcpTool := internalmcp.NewTool(tool.ToolName, tool.ToolDescription, tool.ToolSchema)
		mcpTool.Annotations = tool.ToolAnnotations
		server.AddTool(mcpTool, tool.ToolHandler)
	}

	return &MCPSdkServerConfig{
		Type:     MCPServerTypeSDK,
		Name:     name,
		Instance: server,
	}
}
