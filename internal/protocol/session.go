package protocol

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/config"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/hook"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/mcp"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/permission"
	"github.com/ethpandaops/claude-agent-sdk-go/internal/userinput"
)

const (
	// defaultInitializeTimeout is the default timeout for initialize control requests.
	defaultInitializeTimeout = 60 * time.Second
)

// Session encapsulates protocol handling logic for hooks, MCP servers, and callbacks.
// It can be used by both Client and Query() to provide protocol support.
type Session struct {
	log        *slog.Logger
	controller *Controller
	options    *config.Options

	// Hook callback storage (callback_id -> callback function)
	hookCallbacks   map[string]hook.Callback
	nextCallbackID  int
	hookCallbacksMu sync.RWMutex

	// SDK MCP servers (keyed by server name)
	sdkMcpServers map[string]mcp.ServerInstance

	// Server initialization result (protected by initMu)
	initMu               sync.RWMutex
	initializationResult map[string]any
}

// NewSession creates a new Session for protocol handling.
func NewSession(
	log *slog.Logger,
	controller *Controller,
	options *config.Options,
) *Session {
	return &Session{
		log:           log.With("component", "session"),
		controller:    controller,
		options:       options,
		hookCallbacks: make(map[string]hook.Callback, 16),
		sdkMcpServers: make(map[string]mcp.ServerInstance, 4),
	}
}

// RegisterHandlers registers protocol handlers for hooks, MCP, and tool permissions.
// This must be called before Initialize().
func (s *Session) RegisterHandlers() {
	s.controller.RegisterHandler("hook_callback", s.HandleHookCallback)
	s.controller.RegisterHandler("mcp_message", s.HandleMCPMessage)
	s.controller.RegisterHandler("can_use_tool", s.HandleCanUseTool)
}

// RegisterMCPServers extracts and registers SDK MCP servers from options.
func (s *Session) RegisterMCPServers() {
	if s.options == nil || s.options.MCPServers == nil {
		return
	}

	for serverKey, serverConfig := range s.options.MCPServers {
		if serverConfig == nil {
			continue
		}

		if sdkConfig, ok := serverConfig.(*mcp.SdkServerConfig); ok {
			if sdkConfig.Instance != nil {
				if server, ok := sdkConfig.Instance.(mcp.ServerInstance); ok {
					s.sdkMcpServers[serverKey] = server
					s.log.Debug("Registered SDK MCP server", "server", serverKey)
				}
			}
		}
	}
}

// Initialize sends the initialization control request to the CLI.
// It generates callback IDs for each hook and stores them for later lookup.
func (s *Session) Initialize(ctx context.Context) error {
	s.log.Debug("Sending initialize request")

	// Build hooks configuration for initialization with callback IDs
	hooksConfig := make(map[string]any, 8)

	if s.options != nil && s.options.Hooks != nil {
		s.hookCallbacksMu.Lock()

		for event, matchers := range s.options.Hooks {
			eventMatchers := make([]map[string]any, 0, len(matchers))

			for _, m := range matchers {
				// Generate callback IDs for each hook in this matcher
				callbackIDs := make([]string, 0, len(m.Hooks))

				for _, hookFn := range m.Hooks {
					callbackID := fmt.Sprintf("hook_%d", s.nextCallbackID)
					s.nextCallbackID++
					s.hookCallbacks[callbackID] = hookFn
					callbackIDs = append(callbackIDs, callbackID)
				}

				matcherConfig := map[string]any{
					"matcher":         m.Matcher,
					"hookCallbackIds": callbackIDs,
				}

				if m.Timeout != nil {
					matcherConfig["timeout"] = *m.Timeout
				}

				eventMatchers = append(eventMatchers, matcherConfig)
			}

			hooksConfig[string(event)] = eventMatchers
		}

		s.hookCallbacksMu.Unlock()
	}

	payload := map[string]any{
		"hooks": hooksConfig,
	}

	// Include agent definitions in the initialize payload (avoids ARG_MAX limits)
	if s.options != nil && len(s.options.Agents) > 0 {
		payload["agents"] = s.options.Agents
	}

	timeout := s.getInitializeTimeout()

	resp, err := s.controller.SendRequest(ctx, "initialize", payload, timeout)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	s.initMu.Lock()
	s.initializationResult = resp.Payload()
	s.initMu.Unlock()

	return nil
}

// getInitializeTimeout returns the initialize timeout from options, env var, or default.
func (s *Session) getInitializeTimeout() time.Duration {
	// Check options for explicit timeout
	if s.options != nil && s.options.InitializeTimeout != nil {
		return *s.options.InitializeTimeout
	}

	// Fall back to env var
	if timeoutStr := os.Getenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT"); timeoutStr != "" {
		if timeoutSec, err := strconv.Atoi(timeoutStr); err == nil && timeoutSec > 0 {
			return time.Duration(timeoutSec) * time.Second
		}
	}

	// Fall back to default
	return defaultInitializeTimeout
}

// NeedsInitialization returns true if the session has callbacks that require initialization.
func (s *Session) NeedsInitialization() bool {
	if s.options == nil {
		return false
	}

	// Need initialization if we have hooks, CanUseTool callback, SDK MCP servers, or agents
	return len(s.options.Hooks) > 0 ||
		s.options.CanUseTool != nil ||
		s.options.OnUserInput != nil ||
		len(s.sdkMcpServers) > 0 ||
		len(s.options.Agents) > 0
}

// GetInitializationResult returns a copy of the server initialization info.
// Returns nil if not initialized.
func (s *Session) GetInitializationResult() map[string]any {
	s.initMu.RLock()
	defer s.initMu.RUnlock()

	if s.initializationResult == nil {
		return nil
	}

	// Return a defensive copy to prevent caller mutation
	return maps.Clone(s.initializationResult)
}

// GetSDKMCPServerNames returns the names of all registered in-process SDK MCP servers.
func (s *Session) GetSDKMCPServerNames() []string {
	names := make([]string, 0, len(s.sdkMcpServers))
	for name := range s.sdkMcpServers {
		names = append(names, name)
	}

	return names
}

// HandleHookCallback handles hook_callback control requests from the CLI.
// The CLI sends callback_id which we use to look up the registered callback.
func (s *Session) HandleHookCallback(
	ctx context.Context,
	req *ControlRequest,
) (map[string]any, error) {
	// Check for cancellation before processing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.log.Debug("Handling hook callback")

	// Extract fields from request
	callbackID, _ := req.Request["callback_id"].(string)
	inputData, _ := req.Request["input"].(map[string]any)
	toolUseID, _ := req.Request["tool_use_id"].(*string)

	// Handle tool_use_id as string (JSON may have it as string, not *string)
	if toolUseID == nil {
		if toolUseIDStr, ok := req.Request["tool_use_id"].(string); ok && toolUseIDStr != "" {
			toolUseID = &toolUseIDStr
		}
	}

	// Look up callback by ID
	s.hookCallbacksMu.RLock()
	callback, exists := s.hookCallbacks[callbackID]
	s.hookCallbacksMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown callback_id: %s", callbackID)
	}

	// Parse input into appropriate HookInput type
	hookInput, err := s.parseHookInput(inputData)
	if err != nil {
		return nil, fmt.Errorf("parse hook input: %w", err)
	}

	// Call the callback
	hookCtx := &hook.Context{}

	output, err := callback(ctx, hookInput, toolUseID, hookCtx)
	if err != nil {
		return nil, fmt.Errorf("hook callback error: %w", err)
	}

	// Convert output to response format
	return s.convertHookOutput(output)
}

// parseHookInput converts the input map to the appropriate HookInput type.
func (s *Session) parseHookInput(inputData map[string]any) (hook.Input, error) {
	if inputData == nil {
		return nil, fmt.Errorf("input data is nil")
	}

	hookEventName, _ := inputData["hook_event_name"].(string)
	sessionID, _ := inputData["session_id"].(string)
	transcriptPath, _ := inputData["transcript_path"].(string)
	cwd, _ := inputData["cwd"].(string)

	var permissionMode *string
	if pm, ok := inputData["permission_mode"].(string); ok {
		permissionMode = &pm
	}

	baseInput := hook.BaseInput{
		SessionID:      sessionID,
		TranscriptPath: transcriptPath,
		Cwd:            cwd,
		PermissionMode: permissionMode,
	}

	switch hookEventName {
	case string(hook.EventPreToolUse):
		toolName, _ := inputData["tool_name"].(string)
		toolInput, _ := inputData["tool_input"].(map[string]any)
		toolUseID, _ := inputData["tool_use_id"].(string)

		return &hook.PreToolUseInput{
			BaseInput:     baseInput,
			HookEventName: hookEventName,
			ToolName:      toolName,
			ToolInput:     toolInput,
			ToolUseID:     toolUseID,
		}, nil

	case string(hook.EventPostToolUse):
		toolName, _ := inputData["tool_name"].(string)
		toolInput, _ := inputData["tool_input"].(map[string]any)
		toolUseID, _ := inputData["tool_use_id"].(string)
		toolResponse := inputData["tool_response"]

		return &hook.PostToolUseInput{
			BaseInput:     baseInput,
			HookEventName: hookEventName,
			ToolName:      toolName,
			ToolInput:     toolInput,
			ToolUseID:     toolUseID,
			ToolResponse:  toolResponse,
		}, nil

	case string(hook.EventUserPromptSubmit):
		prompt, _ := inputData["prompt"].(string)

		return &hook.UserPromptSubmitInput{
			BaseInput:     baseInput,
			HookEventName: hookEventName,
			Prompt:        prompt,
		}, nil

	case string(hook.EventStop):
		stopHookActive, _ := inputData["stop_hook_active"].(bool)

		return &hook.StopInput{
			BaseInput:      baseInput,
			HookEventName:  hookEventName,
			StopHookActive: stopHookActive,
		}, nil

	case string(hook.EventSubagentStop):
		stopHookActive, _ := inputData["stop_hook_active"].(bool)
		agentID, _ := inputData["agent_id"].(string)
		agentTranscriptPath, _ := inputData["agent_transcript_path"].(string)
		agentType, _ := inputData["agent_type"].(string)

		return &hook.SubagentStopInput{
			BaseInput:           baseInput,
			HookEventName:       hookEventName,
			StopHookActive:      stopHookActive,
			AgentID:             agentID,
			AgentTranscriptPath: agentTranscriptPath,
			AgentType:           agentType,
		}, nil

	case string(hook.EventPreCompact):
		trigger, _ := inputData["trigger"].(string)

		var customInstructions *string
		if ci, ok := inputData["custom_instructions"].(string); ok && ci != "" {
			customInstructions = &ci
		}

		return &hook.PreCompactInput{
			BaseInput:          baseInput,
			HookEventName:      hookEventName,
			Trigger:            trigger,
			CustomInstructions: customInstructions,
		}, nil

	case string(hook.EventPostToolUseFailure):
		toolName, _ := inputData["tool_name"].(string)
		toolInput, _ := inputData["tool_input"].(map[string]any)
		toolUseID, _ := inputData["tool_use_id"].(string)
		toolError, _ := inputData["error"].(string)

		var isInterrupt *bool
		if v, ok := inputData["is_interrupt"].(bool); ok {
			isInterrupt = &v
		}

		return &hook.PostToolUseFailureInput{
			BaseInput:     baseInput,
			HookEventName: hookEventName,
			ToolName:      toolName,
			ToolInput:     toolInput,
			ToolUseID:     toolUseID,
			Error:         toolError,
			IsInterrupt:   isInterrupt,
		}, nil

	case string(hook.EventNotification):
		msg, _ := inputData["message"].(string)
		notificationType, _ := inputData["notification_type"].(string)

		var title *string
		if t, ok := inputData["title"].(string); ok && t != "" {
			title = &t
		}

		return &hook.NotificationInput{
			BaseInput:        baseInput,
			HookEventName:    hookEventName,
			Message:          msg,
			Title:            title,
			NotificationType: notificationType,
		}, nil

	case string(hook.EventSubagentStart):
		agentID, _ := inputData["agent_id"].(string)
		agentType, _ := inputData["agent_type"].(string)

		return &hook.SubagentStartInput{
			BaseInput:     baseInput,
			HookEventName: hookEventName,
			AgentID:       agentID,
			AgentType:     agentType,
		}, nil

	case string(hook.EventPermissionRequest):
		toolName, _ := inputData["tool_name"].(string)
		toolInput, _ := inputData["tool_input"].(map[string]any)

		var permissionSuggestions []any
		if ps, ok := inputData["permission_suggestions"].([]any); ok {
			permissionSuggestions = ps
		}

		return &hook.PermissionRequestInput{
			BaseInput:             baseInput,
			HookEventName:         hookEventName,
			ToolName:              toolName,
			ToolInput:             toolInput,
			PermissionSuggestions: permissionSuggestions,
		}, nil

	default:
		// Unknown event type - return a generic stop input as fallback
		return &hook.StopInput{
			BaseInput:      baseInput,
			HookEventName:  hookEventName,
			StopHookActive: false,
		}, nil
	}
}

// convertHookOutput converts a HookJSONOutput to a response map for the CLI.
func (s *Session) convertHookOutput(output hook.JSONOutput) (map[string]any, error) {
	if output == nil {
		// Default: continue with no special output
		return map[string]any{
			"continue": true,
		}, nil
	}

	switch o := output.(type) {
	case *hook.SyncJSONOutput:
		result := make(map[string]any, 8)

		if o.Continue != nil {
			result["continue"] = *o.Continue
		} else {
			result["continue"] = true
		}

		if o.SuppressOutput != nil {
			result["suppressOutput"] = *o.SuppressOutput
		}

		if o.StopReason != nil {
			result["stopReason"] = *o.StopReason
		}

		if o.Decision != nil {
			result["decision"] = *o.Decision
		}

		if o.SystemMessage != nil {
			result["systemMessage"] = *o.SystemMessage
		}

		if o.Reason != nil {
			result["reason"] = *o.Reason
		}

		if o.HookSpecificOutput != nil {
			result["hookSpecificOutput"] = o.HookSpecificOutput
		}

		return result, nil

	case *hook.AsyncJSONOutput:
		return map[string]any{
			"async":        o.Async,
			"asyncTimeout": o.AsyncTimeout,
		}, nil

	default:
		return map[string]any{
			"continue": true,
		}, nil
	}
}

// HandleMCPMessage handles unified mcp_message control requests from the CLI.
// Routes based on the JSONRPC method field: initialize, tools/list, tools/call,
// notifications/initialized.
func (s *Session) HandleMCPMessage(
	ctx context.Context,
	req *ControlRequest,
) (map[string]any, error) {
	// Check for cancellation before processing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.log.Debug("Handling MCP message request")

	// Extract server_name and message from request
	serverName, _ := req.Request["server_name"].(string)
	message, _ := req.Request["message"].(map[string]any)

	if message == nil {
		return nil, fmt.Errorf("missing message field in mcp_message request")
	}

	// Extract JSONRPC fields
	jsonrpcVersion, _ := message["jsonrpc"].(string)
	method, _ := message["method"].(string)
	params, _ := message["params"].(map[string]any)

	// Get message ID (can be string or number)
	var msgID any
	if id, ok := message["id"].(float64); ok {
		msgID = int(id)
	} else if id, ok := message["id"].(string); ok {
		msgID = id
	} else {
		msgID = message["id"]
	}

	// Look up the server
	server, exists := s.sdkMcpServers[serverName]
	if !exists {
		return s.mcpErrorResponse(msgID, -32600, fmt.Sprintf("MCP server not found: %s", serverName)), nil
	}

	// Route based on method
	switch method {
	case "initialize":
		return s.handleMCPInitialize(msgID, jsonrpcVersion, server)

	case "notifications/initialized":
		// No-op acknowledgment - return empty success
		return map[string]any{
			"mcp_response": map[string]any{
				"jsonrpc": "2.0",
				"id":      msgID,
				"result":  map[string]any{},
			},
		}, nil

	case "tools/list":
		return s.handleMCPToolsList(msgID, server)

	case "tools/call":
		return s.handleMCPToolsCall(ctx, msgID, params, server)

	default:
		return s.mcpErrorResponse(msgID, -32601, fmt.Sprintf("Method not found: %s", method)), nil
	}
}

// handleMCPInitialize handles the initialize method.
func (s *Session) handleMCPInitialize(
	msgID any,
	_ string,
	server mcp.ServerInstance,
) (map[string]any, error) {
	// Get server info if the server supports it
	var serverInfo map[string]any

	var capabilities map[string]any

	if infoProvider, ok := server.(interface{ ServerInfo() map[string]any }); ok {
		serverInfo = infoProvider.ServerInfo()
	} else {
		serverInfo = map[string]any{
			"name":    "sdk-server",
			"version": "1.0.0",
		}
	}

	if capsProvider, ok := server.(interface{ Capabilities() map[string]any }); ok {
		capabilities = capsProvider.Capabilities()
	} else {
		capabilities = map[string]any{
			"tools": map[string]any{},
		}
	}

	return map[string]any{
		"mcp_response": map[string]any{
			"jsonrpc": "2.0",
			"id":      msgID,
			"result": map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    capabilities,
				"serverInfo":      serverInfo,
			},
		},
	}, nil
}

// handleMCPToolsList handles the tools/list method.
func (s *Session) handleMCPToolsList(
	msgID any,
	server mcp.ServerInstance,
) (map[string]any, error) {
	tools := server.ListTools()

	return map[string]any{
		"mcp_response": map[string]any{
			"jsonrpc": "2.0",
			"id":      msgID,
			"result": map[string]any{
				"tools": tools,
			},
		},
	}, nil
}

// handleMCPToolsCall handles the tools/call method.
func (s *Session) handleMCPToolsCall(
	ctx context.Context,
	msgID any,
	params map[string]any,
	server mcp.ServerInstance,
) (map[string]any, error) {
	if params == nil {
		return s.mcpErrorResponse(msgID, -32602, "Missing params for tools/call"), nil
	}

	toolName, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]any)

	if toolName == "" {
		return s.mcpErrorResponse(msgID, -32602, "Missing tool name in params"), nil
	}

	result, err := server.CallTool(ctx, toolName, arguments)
	if err != nil {
		return s.mcpErrorResponse(msgID, -32603, err.Error()), nil
	}

	return map[string]any{
		"mcp_response": map[string]any{
			"jsonrpc": "2.0",
			"id":      msgID,
			"result":  result,
		},
	}, nil
}

// mcpErrorResponse creates a JSONRPC error response.
func (s *Session) mcpErrorResponse(msgID any, code int, message string) map[string]any {
	return map[string]any{
		"mcp_response": map[string]any{
			"jsonrpc": "2.0",
			"id":      msgID,
			"error": map[string]any{
				"code":    code,
				"message": message,
			},
		},
	}
}

// HandleCanUseTool is called by CLI before tool use.
func (s *Session) HandleCanUseTool(
	ctx context.Context,
	req *ControlRequest,
) (map[string]any, error) {
	// Check for cancellation before processing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Extract from nested request
	toolName, _ := req.Request["tool_name"].(string)
	input, _ := req.Request["input"].(map[string]any)

	// AskUserQuestion can be handled directly through OnUserInput without requiring
	// callers to manually inspect tool_use blocks and send tool_result messages.
	if result, handled := s.handleAskUserQuestion(ctx, toolName, input); handled {
		return result, nil
	}

	// If no permission callback configured, allow by default.
	if s.options == nil || s.options.CanUseTool == nil {
		return map[string]any{
			"behavior":     "allow",
			"updatedInput": make(map[string]any),
		}, nil
	}

	// Extract suggestions from nested request if present
	var suggestions []*permission.Update
	if suggestionsData, ok := req.Request["suggestions"].([]any); ok {
		suggestions = make([]*permission.Update, 0, len(suggestionsData))

		for _, sg := range suggestionsData {
			if suggestionMap, ok := sg.(map[string]any); ok {
				// Parse suggestion into PermissionUpdate
				// This is a simplified version - may need more sophisticated parsing
				update := &permission.Update{}
				if t, ok := suggestionMap["type"].(string); ok {
					update.Type = permission.UpdateType(t)
				}

				suggestions = append(suggestions, update)
			}
		}
	}

	// Create permission context with 4th parameter
	permCtx := &permission.Context{
		Suggestions: suggestions,
	}

	decision, err := s.options.CanUseTool(ctx, toolName, input, permCtx)
	if err != nil {
		return nil, err
	}

	// Type assert to access fields based on the concrete type
	switch d := decision.(type) {
	case *permission.ResultAllow:
		updatedInput := d.UpdatedInput
		if updatedInput == nil {
			updatedInput = make(map[string]any)
		}

		result := map[string]any{
			"behavior":     "allow",
			"updatedInput": updatedInput,
		}

		if d.UpdatedPermissions != nil {
			// Convert to CLI format
			updates := make([]map[string]any, len(d.UpdatedPermissions))
			for i, u := range d.UpdatedPermissions {
				updates[i] = u.ToDict()
			}

			result["updatedPermissions"] = updates
		}

		return result, nil

	case *permission.ResultDeny:
		result := map[string]any{
			"behavior": "deny",
			"message":  d.Message,
		}

		if d.Interrupt {
			result["interrupt"] = true
		}

		return result, nil

	default:
		return nil, fmt.Errorf(
			"tool permission callback must return *PermissionResultAllow or *PermissionResultDeny, got %T",
			decision,
		)
	}
}

func (s *Session) handleAskUserQuestion(
	ctx context.Context,
	toolName string,
	input map[string]any,
) (map[string]any, bool) {
	if toolName != "AskUserQuestion" || s.options == nil || s.options.OnUserInput == nil {
		return nil, false
	}

	parsedReq, parseErr := parseAskUserInput(input)
	if parseErr != nil {
		return denyToolUse("on_user_input parse error: " + parseErr.Error()), true
	}

	resp, callbackErr := s.options.OnUserInput(ctx, parsedReq)
	if callbackErr != nil {
		return denyToolUse("on_user_input callback error: " + callbackErr.Error()), true
	}

	updatedInput, applyErr := applyAskUserAnswers(input, parsedReq, resp)
	if applyErr != nil {
		return denyToolUse("on_user_input invalid response: " + applyErr.Error()), true
	}

	return map[string]any{
		"behavior":     "allow",
		"updatedInput": updatedInput,
	}, true
}

func denyToolUse(message string) map[string]any {
	return map[string]any{
		"behavior": "deny",
		"message":  message,
	}
}

func parseAskUserInput(input map[string]any) (*userinput.Request, error) {
	if input == nil {
		return nil, fmt.Errorf("missing tool input")
	}

	rawQuestions, ok := input["questions"].([]any)
	if !ok || len(rawQuestions) == 0 {
		return nil, fmt.Errorf("missing questions")
	}

	req := &userinput.Request{
		Questions: make([]userinput.Question, 0, len(rawQuestions)),
	}

	for _, rawQuestion := range rawQuestions {
		questionMap, ok := rawQuestion.(map[string]any)
		if !ok {
			continue
		}

		question := userinput.Question{}
		question.Question, _ = questionMap["question"].(string)
		question.Header, _ = questionMap["header"].(string)

		question.MultiSelect, _ = questionMap["multiSelect"].(bool)
		if !question.MultiSelect {
			if multiSelectSnake, ok := questionMap["multi_select"].(bool); ok {
				question.MultiSelect = multiSelectSnake
			}
		}

		if rawOptions, ok := questionMap["options"].([]any); ok {
			question.Options = make([]userinput.QuestionOption, 0, len(rawOptions))

			for _, rawOption := range rawOptions {
				optionMap, ok := rawOption.(map[string]any)
				if !ok {
					continue
				}

				option := userinput.QuestionOption{}
				option.Label, _ = optionMap["label"].(string)
				option.Description, _ = optionMap["description"].(string)
				question.Options = append(question.Options, option)
			}
		}

		req.Questions = append(req.Questions, question)
	}

	if len(req.Questions) == 0 {
		return nil, fmt.Errorf("no parseable questions")
	}

	return req, nil
}

func applyAskUserAnswers(
	input map[string]any,
	req *userinput.Request,
	resp *userinput.Response,
) (map[string]any, error) {
	if resp == nil || len(resp.Answers) == 0 {
		return nil, fmt.Errorf("answers cannot be empty")
	}

	answers := make(map[string]any, len(resp.Answers))

	for _, question := range req.Questions {
		answer, ok := resp.Answers[question.Question]
		if !ok || answer == nil || len(answer.Answers) == 0 {
			continue
		}

		answerList := make([]string, 0, len(answer.Answers))
		for _, value := range answer.Answers {
			if value != "" {
				answerList = append(answerList, value)
			}
		}

		if len(answerList) > 0 {
			answers[question.Question] = answerList
		}
	}

	if len(answers) == 0 {
		return nil, fmt.Errorf("no valid answers matched question text")
	}

	updatedInput := make(map[string]any, len(input)+1)
	maps.Copy(updatedInput, input)
	updatedInput["answers"] = answers

	return updatedInput, nil
}
