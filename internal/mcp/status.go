package mcp

// ServerConnectionStatus represents the connection state of an MCP server.
type ServerConnectionStatus string

const (
	// ServerConnectionStatusConnected indicates the server is connected.
	ServerConnectionStatusConnected ServerConnectionStatus = "connected"
	// ServerConnectionStatusFailed indicates the server failed to connect.
	ServerConnectionStatusFailed ServerConnectionStatus = "failed"
	// ServerConnectionStatusNeedsAuth indicates the server requires authentication.
	ServerConnectionStatusNeedsAuth ServerConnectionStatus = "needs-auth"
	// ServerConnectionStatusPending indicates the server is still connecting.
	ServerConnectionStatusPending ServerConnectionStatus = "pending"
	// ServerConnectionStatusDisabled indicates the server is disabled.
	ServerConnectionStatusDisabled ServerConnectionStatus = "disabled"
)

// ToolAnnotations describes MCP tool capability metadata.
type ToolAnnotations struct {
	ReadOnly    bool `json:"readOnly,omitempty"`
	Destructive bool `json:"destructive,omitempty"`
	OpenWorld   bool `json:"openWorld,omitempty"`
}

// ToolInfo describes a tool exposed by an MCP server.
type ToolInfo struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ServerInfo describes the MCP server information from initialize.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerStatusConfig captures the serializable server config included in mcp_status responses.
type ServerStatusConfig struct {
	Type    string            `json:"type"`
	Name    string            `json:"name,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	ID      string            `json:"id,omitempty"`
}

// ServerStatus represents the connection status of a single MCP server.
type ServerStatus struct {
	Name       string                 `json:"name"`
	Status     ServerConnectionStatus `json:"status"`
	ServerInfo *ServerInfo            `json:"serverInfo,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Config     *ServerStatusConfig    `json:"config,omitempty"`
	Scope      string                 `json:"scope,omitempty"`
	Tools      []ToolInfo             `json:"tools,omitempty"`
}

// Status represents the connection status of all configured MCP servers.
type Status struct {
	MCPServers []ServerStatus `json:"mcpServers"`
}
