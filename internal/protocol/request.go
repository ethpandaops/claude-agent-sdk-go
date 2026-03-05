package protocol

import (
	"context"
)

// ControlRequest represents a control message sent to or received from the CLI.
//
// Wire format:
//
//	{
//	  "type": "control_request",
//	  "request_id": "req_1_abc123",
//	  "request": {
//	    "subtype": "initialize",
//	    "hooks": {...}
//	  }
//	}
type ControlRequest struct {
	// Type is always "control_request"
	Type string `json:"type"`

	// RequestID uniquely identifies this request for response correlation
	RequestID string `json:"request_id"` //nolint:tagliatelle // Claude CLI uses snake_case

	// Request contains the nested request data including subtype and payload fields
	Request map[string]any `json:"request"`
}

// Subtype extracts the subtype from the nested request data.
func (r *ControlRequest) Subtype() string {
	if s, ok := r.Request["subtype"].(string); ok {
		return s
	}

	return ""
}

// ControlResponse represents a response to a control request.
//
// Wire format for success:
//
//	{
//	  "type": "control_response",
//	  "response": {
//	    "subtype": "success",
//	    "request_id": "req_1_abc123",
//	    "response": {...}
//	  }
//	}
//
// Wire format for error:
//
//	{
//	  "type": "control_response",
//	  "response": {
//	    "subtype": "error",
//	    "request_id": "req_1_abc123",
//	    "error": "error message"
//	  }
//	}
type ControlResponse struct {
	// Type is always "control_response"
	Type string `json:"type"`

	// Response contains the nested response data including subtype, request_id,
	// and either response (for success) or error (for error)
	Response map[string]any `json:"response"`
}

// IsError checks if the response is an error response.
func (r *ControlResponse) IsError() bool {
	if s, ok := r.Response["subtype"].(string); ok {
		return s == "error"
	}

	return false
}

// ErrorMessage extracts the error message from an error response.
func (r *ControlResponse) ErrorMessage() string {
	if e, ok := r.Response["error"].(string); ok {
		return e
	}

	return ""
}

// Payload extracts the response payload from a success response.
func (r *ControlResponse) Payload() map[string]any {
	if p, ok := r.Response["response"].(map[string]any); ok {
		return p
	}

	return nil
}

// RequestID extracts the request_id from the nested response.
func (r *ControlResponse) RequestID() string {
	if id, ok := r.Response["request_id"].(string); ok {
		return id
	}

	return ""
}

// RequestHandler is a function that handles incoming control requests from the CLI.
//
// Handlers are registered for specific subtypes and are invoked when the CLI
// sends a control_request (e.g., for hook callbacks).
//
// The handler should return a payload map or an error.
// The Controller automatically wraps the result in a ControlResponse.
type RequestHandler func(ctx context.Context, req *ControlRequest) (map[string]any, error)
