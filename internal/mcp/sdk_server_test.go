package mcp

import (
	"context"
	"errors"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestSDKServerMetadata(t *testing.T) {
	server := NewSDKServer("demo", "1.2.3")

	require.Equal(t, "demo", server.Name())
	require.Equal(t, "1.2.3", server.Version())
	require.Equal(t, map[string]any{
		"name":    "demo",
		"version": "1.2.3",
	}, server.ServerInfo())
	require.Equal(t, map[string]any{
		"tools": map[string]any{},
	}, server.Capabilities())
}

func TestSDKServerListToolsAndCallTool(t *testing.T) {
	server := NewSDKServer("demo", "1.0.0")
	schema := SimpleSchema(map[string]string{"text": "string"})
	server.AddTool(
		NewTool("echo", "echoes text", schema),
		func(_ context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			args, err := ParseArguments(req)
			if err != nil {
				return nil, err
			}

			text, _ := args["text"].(string)

			return TextResult("echo: " + text), nil
		},
	)

	tools := server.ListTools()
	require.Len(t, tools, 1)
	require.Equal(t, "echo", tools[0]["name"])
	require.Equal(t, "echoes text", tools[0]["description"])

	inputSchema, ok := tools[0]["inputSchema"].(map[string]any)
	require.True(t, ok, "expected inputSchema to be serialized as a map")
	require.Equal(t, "object", inputSchema["type"])

	result, err := server.CallTool(context.Background(), "echo", map[string]any{"text": "hello"})
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": "echo: hello",
			},
		},
	}, result)

	missing, err := server.CallTool(context.Background(), "unknown", map[string]any{})
	require.NoError(t, err)
	require.Equal(t, true, missing["is_error"])
}

func TestSDKServerCallTool_HandlerError(t *testing.T) {
	server := NewSDKServer("demo", "1.0.0")
	server.AddTool(
		NewTool("fails", "always fails", nil),
		func(_ context.Context, _ *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return nil, errors.New("boom")
		},
	)

	result, err := server.CallTool(context.Background(), "fails", map[string]any{})

	require.NoError(t, err)
	require.Equal(t, true, result["is_error"])
}

func TestConvertCallToolResultToMap(t *testing.T) {
	t.Run("nil result returns empty content", func(t *testing.T) {
		require.Equal(t, map[string]any{
			"content": []map[string]any{},
		}, convertCallToolResultToMap(nil))
	})

	t.Run("mixed content is converted to protocol maps", func(t *testing.T) {
		result := &mcpgo.CallToolResult{
			Content: []mcpgo.Content{
				&mcpgo.TextContent{Text: "hello"},
				&mcpgo.ImageContent{Data: []byte("img"), MIMEType: "image/png"},
				&mcpgo.AudioContent{Data: []byte("aud"), MIMEType: "audio/wav"},
				&mcpgo.ResourceLink{URI: "file:///a.txt", Name: "a.txt"},
				&mcpgo.EmbeddedResource{
					Resource: &mcpgo.ResourceContents{
						URI:      "file:///b.txt",
						MIMEType: "text/plain",
						Text:     "body",
					},
				},
			},
			IsError: true,
		}

		got := convertCallToolResultToMap(result)
		content, ok := got["content"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, content, 5)
		require.Equal(t, true, got["is_error"])
		require.Equal(t, "text", content[0]["type"])
		require.Equal(t, "hello", content[0]["text"])
		require.Equal(t, "image", content[1]["type"])
		require.Equal(t, "audio", content[2]["type"])
		require.Equal(t, "resource_link", content[3]["type"])
		require.Equal(t, "resource", content[4]["type"])
	})
}

func TestSimpleSchema(t *testing.T) {
	schema := SimpleSchema(map[string]string{
		"name":   "string",
		"active": "bool",
		"scores": "[]float64",
	})

	require.Equal(t, "object", schema.Type)
	require.ElementsMatch(t, []string{"name", "active", "scores"}, schema.Required)
	require.Equal(t, "string", schema.Properties["name"].Type)
	require.Equal(t, "boolean", schema.Properties["active"].Type)
	require.Equal(t, "array", schema.Properties["scores"].Type)
	require.Equal(t, "number", schema.Properties["scores"].Items.Type)
}

func TestGoTypeToJSONSchema(t *testing.T) {
	tests := []struct {
		name      string
		goType    string
		wantType  string
		wantItems *string
	}{
		{
			name:     "string",
			goType:   "string",
			wantType: "string",
		},
		{
			name:     "integer",
			goType:   "int64",
			wantType: "integer",
		},
		{
			name:     "number",
			goType:   "float32",
			wantType: "number",
		},
		{
			name:     "boolean",
			goType:   "boolean",
			wantType: "boolean",
		},
		{
			name:     "object",
			goType:   "map[string]any",
			wantType: "object",
		},
		{
			name:      "array",
			goType:    "[]int",
			wantType:  "array",
			wantItems: strPtr("integer"),
		},
		{
			name:     "fallback",
			goType:   "customType",
			wantType: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := goTypeToJSONSchema(tt.goType)

			require.Equal(t, tt.wantType, got.Type)

			if tt.wantItems != nil {
				require.NotNil(t, got.Items)
				require.Equal(t, *tt.wantItems, got.Items.Type)
			}
		})
	}
}

func TestResultHelpersAndNewTool(t *testing.T) {
	textResult := TextResult("ok")
	require.False(t, textResult.IsError)
	require.Len(t, textResult.Content, 1)

	errorResult := ErrorResult("failed")
	require.True(t, errorResult.IsError)
	require.Len(t, errorResult.Content, 1)

	imageResult := ImageResult([]byte("bin"), "image/png")
	require.False(t, imageResult.IsError)
	require.Len(t, imageResult.Content, 1)

	schema := SimpleSchema(map[string]string{"x": "int"})
	tool := NewTool("sum", "adds values", schema)
	require.Equal(t, "sum", tool.Name)
	require.Equal(t, "adds values", tool.Description)
	require.Equal(t, schema, tool.InputSchema)
}

func TestParseArguments(t *testing.T) {
	t.Run("nil request and empty args return empty map", func(t *testing.T) {
		args, err := ParseArguments(nil)
		require.NoError(t, err)
		require.Empty(t, args)

		args, err = ParseArguments(&mcpgo.CallToolRequest{Params: &mcpgo.CallToolParamsRaw{}})
		require.NoError(t, err)
		require.Empty(t, args)
	})

	t.Run("valid arguments are parsed", func(t *testing.T) {
		req := &mcpgo.CallToolRequest{
			Params: &mcpgo.CallToolParamsRaw{
				Arguments: []byte(`{"name":"claude","count":3}`),
			},
		}

		args, err := ParseArguments(req)
		require.NoError(t, err)
		require.Equal(t, "claude", args["name"])
		require.Equal(t, float64(3), args["count"])
	})

	t.Run("invalid json returns wrapped error", func(t *testing.T) {
		req := &mcpgo.CallToolRequest{
			Params: &mcpgo.CallToolParamsRaw{
				Arguments: []byte(`{"name":`),
			},
		}

		args, err := ParseArguments(req)
		require.Error(t, err)
		require.Nil(t, args)
		require.Contains(t, err.Error(), "failed to unmarshal arguments")
	})
}

func strPtr(s string) *string {
	return &s
}
