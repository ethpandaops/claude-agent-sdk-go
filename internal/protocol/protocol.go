package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/ethpandaops/claude-agent-sdk-go/internal/errors"
	"github.com/oklog/ulid/v2"
)

// Transport defines the minimal interface needed for protocol operations.
//
// This interface is satisfied by the CLITransport but allows for testing
// with mock transports.
type Transport interface {
	ReadMessages(ctx context.Context) (<-chan map[string]any, <-chan error)
	SendMessage(ctx context.Context, data []byte) error
}

// Controller manages bidirectional control message communication with the Claude CLI.
//
// The Controller handles:
//   - Sending control_request messages with unique request IDs
//   - Receiving and routing control_response messages to waiting requests
//   - Request timeout enforcement
//   - Handler registration for incoming control_request messages from the CLI
//   - Forwarding non-control messages to consumers via the Messages channel
//
// The Controller must be started with Start() before use and manages its own
// goroutine for reading and routing messages.
type Controller struct {
	log       *slog.Logger
	transport Transport

	// Request tracking
	pendingMu sync.RWMutex
	pending   map[string]*pendingRequest

	// In-flight operation tracking for cancellation support
	inFlightMu sync.RWMutex
	inFlight   map[string]*inFlightOperation

	// Handler registry for incoming requests
	handlersMu sync.RWMutex
	handlers   map[string]RequestHandler

	// Non-control messages forwarded to consumers
	messages chan map[string]any

	// Fatal error handling - stores error and broadcasts via done channel
	errMu    sync.RWMutex
	fatalErr error

	// Lifecycle management
	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup
}

// pendingRequest tracks an outgoing request awaiting response.
type pendingRequest struct {
	subtype  string
	response chan *ControlResponse
	timeout  time.Time
}

// inFlightOperation tracks an incoming control request being handled.
type inFlightOperation struct {
	requestID string
	subtype   string
	cancel    context.CancelFunc
	startTime time.Time
	completed bool
}

// NewController creates a new protocol controller.
//
// The logger will receive debug, info, warn, and error messages during
// protocol operations. The transport must be connected before calling Start().
func NewController(log *slog.Logger, transport Transport) *Controller {
	return &Controller{
		log:       log.With("component", "protocol"),
		transport: transport,
		pending:   make(map[string]*pendingRequest, 10),
		inFlight:  make(map[string]*inFlightOperation, 10),
		handlers:  make(map[string]RequestHandler, 10),
		messages:  make(chan map[string]any, 100), // Buffered to avoid blocking during initialization
		done:      make(chan struct{}),
	}
}

// closeDone safely closes the done channel exactly once.
func (c *Controller) closeDone() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

// SetFatalError stores a fatal error and broadcasts to all waiters by closing done.
func (c *Controller) SetFatalError(err error) {
	c.errMu.Lock()

	if c.fatalErr == nil {
		c.fatalErr = err
	}

	c.errMu.Unlock()

	c.closeDone()
}

// FatalError returns the fatal error if one occurred.
func (c *Controller) FatalError() error {
	c.errMu.RLock()
	defer c.errMu.RUnlock()

	return c.fatalErr
}

// Done returns a channel that is closed when the controller stops.
func (c *Controller) Done() <-chan struct{} {
	return c.done
}

// Start begins reading messages from the transport and routing control messages.
//
// This method spawns a goroutine that reads from the transport and routes
// control_request and control_response messages. The goroutine stops when
// the context is cancelled or the transport is closed.
//
// Start must be called before SendRequest or any handlers will work.
func (c *Controller) Start(ctx context.Context) error {
	c.log.Debug("Starting protocol controller")

	messages, errs := c.transport.ReadMessages(ctx)

	c.wg.Add(1)

	go c.readLoop(ctx, messages, errs)

	c.log.Info("Protocol controller started")

	return nil
}

// Stop gracefully shuts down the controller.
//
// This method signals the read loop to stop, cancels all in-flight operations,
// and waits for completion. It's safe to call Stop multiple times.
func (c *Controller) Stop() {
	c.log.Debug("Stopping protocol controller")

	c.closeDone()

	c.CancelAllInFlight()
	c.wg.Wait()
	c.log.Info("Protocol controller stopped")
}

// Messages returns a channel for receiving non-control messages.
//
// The controller acts as a multiplexer: it reads all messages from the transport,
// handles control messages internally, and forwards regular messages through this
// channel. Consumers should read from this channel instead of calling
// transport.ReadMessages() directly.
//
// The channel is closed when the controller stops or the transport closes.
// Use Done() and FatalError() to detect and retrieve transport errors.
func (c *Controller) Messages() <-chan map[string]any {
	return c.messages
}

// SendRequest sends a control request and waits for the response.
//
// This method generates a unique request ID, sends the control_request,
// and blocks until a matching control_response is received or the timeout
// expires.
//
// The timeout parameter specifies how long to wait for a response.
// Use context cancellation for overall operation timeout.
//
// Returns an error if the request fails to send, times out, or the CLI
// returns an error response.
func (c *Controller) SendRequest(
	ctx context.Context,
	subtype string,
	payload map[string]any,
	timeout time.Duration,
) (*ControlResponse, error) {
	// Generate unique request ID
	requestID := c.generateRequestID()

	c.log.Debug("Sending control request", "request_id", requestID, "subtype", subtype)

	// Create pending request tracker
	responseChan := make(chan *ControlResponse, 1)
	pending := &pendingRequest{
		subtype:  subtype,
		response: responseChan,
		timeout:  time.Now().Add(timeout),
	}

	c.pendingMu.Lock()
	c.pending[requestID] = pending
	c.pendingMu.Unlock()

	// Build nested request structure
	requestPayload := map[string]any{"subtype": subtype}
	maps.Copy(requestPayload, payload)

	req := &ControlRequest{
		Type:      "control_request",
		RequestID: requestID,
		Request:   requestPayload,
	}

	data, err := json.Marshal(req)
	if err != nil {
		c.log.Error("Failed to marshal control request", "error", err)

		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if err := c.transport.SendMessage(ctx, data); err != nil {
		c.log.Error("Failed to send control request", "error", err)

		return nil, fmt.Errorf("send request: %w", err)
	}

	c.log.Debug("Control request sent, waiting for response", "request_id", requestID)

	// Wait for response with timeout
	select {
	case resp := <-responseChan:
		if resp.IsError() {
			errMsg := resp.ErrorMessage()
			c.log.Warn("Control request returned error", "request_id", requestID, "error", errMsg)

			return nil, fmt.Errorf("request error: %s", errMsg)
		}

		c.log.Debug("Received control response", "request_id", requestID)

		return resp, nil

	case <-c.done:
		// Controller stopped (possibly due to transport error) - fail fast
		// Clean up pending request since we're exiting without a response
		c.pendingMu.Lock()
		delete(c.pending, requestID)
		c.pendingMu.Unlock()

		if err := c.FatalError(); err != nil {
			c.log.Warn("Transport error during request", "request_id", requestID, "error", err)

			return nil, fmt.Errorf("transport error: %w", err)
		}

		c.log.Debug("Controller stopped during request", "request_id", requestID)

		return nil, errors.ErrControllerStopped

	case <-time.After(timeout):
		// Clean up pending request since we're exiting without a response
		c.pendingMu.Lock()
		delete(c.pending, requestID)
		c.pendingMu.Unlock()

		c.log.Warn("Control request timed out", "request_id", requestID, "timeout", timeout)

		return nil, fmt.Errorf("%w after %s", errors.ErrRequestTimeout, timeout)

	case <-ctx.Done():
		// Clean up pending request since we're exiting without a response
		c.pendingMu.Lock()
		delete(c.pending, requestID)
		c.pendingMu.Unlock()

		c.log.Debug("Control request cancelled", "request_id", requestID)

		return nil, ctx.Err()
	}
}

// RegisterHandler registers a handler for incoming control requests.
//
// When the CLI sends a control_request with the specified subtype, the handler
// will be invoked. The handler should return a payload map or an error.
//
// Only one handler can be registered per subtype. Registering a handler for
// the same subtype twice will override the previous handler.
func (c *Controller) RegisterHandler(subtype string, handler RequestHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()

	c.log.Debug("Registering control request handler", "subtype", subtype)
	c.handlers[subtype] = handler
}

// readLoop reads messages from the transport and routes control messages.
func (c *Controller) readLoop(
	ctx context.Context,
	messages <-chan map[string]any,
	errs <-chan error,
) {
	defer c.wg.Done()
	defer close(c.messages)
	defer c.log.Debug("Protocol read loop stopped")

	for {
		select {
		case msg, ok := <-messages:
			if !ok {
				c.log.Debug("Message channel closed")

				return
			}

			c.handleMessage(ctx, msg)

		case err, ok := <-errs:
			if !ok {
				c.log.Debug("Error channel closed")

				return
			}

			if err != nil {
				c.log.Debug("Transport error in protocol", "error", err)
				c.SetFatalError(err)

				return
			}

		case <-c.done:
			c.log.Debug("Protocol controller stop signal received")

			return

		case <-ctx.Done():
			c.log.Debug("Context cancelled in protocol read loop")

			return
		}
	}
}

// handleMessage routes a message based on its type.
func (c *Controller) handleMessage(ctx context.Context, msg map[string]any) {
	msgType, _ := msg["type"].(string)

	switch msgType {
	case "control_response":
		c.handleControlResponse(msg)

	case "control_request":
		c.handleControlRequest(ctx, msg)

	case "control_cancel_request":
		c.handleCancelRequest(ctx, msg)

	default:
		// Forward non-control messages to consumers
		select {
		case c.messages <- msg:
		case <-c.done:
		case <-ctx.Done():
		}
	}
}

// handleControlResponse routes a response to the waiting request.
func (c *Controller) handleControlResponse(msg map[string]any) {
	// Extract from nested response
	responseData, ok := msg["response"].(map[string]any)
	if !ok {
		c.log.Warn("Control response missing 'response' field")

		return
	}

	requestID, ok := responseData["request_id"].(string)
	if !ok {
		c.log.Warn("Control response missing request_id in response")

		return
	}

	c.log.Debug("Received control response", "request_id", requestID)

	// Find and claim pending request atomically
	c.pendingMu.Lock()

	pending, exists := c.pending[requestID]
	if exists {
		delete(c.pending, requestID)
	}

	c.pendingMu.Unlock()

	if !exists {
		c.log.Warn("No pending request for control response", "request_id", requestID)

		return
	}

	// Build ControlResponse with nested format
	resp := &ControlResponse{
		Type:     "control_response",
		Response: responseData,
	}

	// Send to waiting goroutine (we own it now, blocking is safe since channel is buffered)
	pending.response <- resp
}

// handleControlRequest invokes the registered handler for an incoming request.
func (c *Controller) handleControlRequest(ctx context.Context, msg map[string]any) {
	// Extract request_id from top level, request data from nested field
	requestID, ok := msg["request_id"].(string)
	if !ok {
		c.log.Warn("Control request missing request_id")

		return
	}

	requestData, ok := msg["request"].(map[string]any)
	if !ok {
		c.log.Warn("Control request missing 'request' field")

		return
	}

	// Build ControlRequest with nested format
	req := &ControlRequest{
		Type:      "control_request",
		RequestID: requestID,
		Request:   requestData,
	}

	subtype := req.Subtype()

	c.log.Debug("Received control request from CLI", "request_id", requestID, "subtype", subtype)

	// Find handler
	c.handlersMu.RLock()
	handler, exists := c.handlers[subtype]
	c.handlersMu.RUnlock()

	if !exists {
		c.log.Warn("No handler registered for control request subtype", "subtype", subtype)
		c.sendErrorResponse(ctx, requestID, "no handler registered")

		return
	}

	// Create cancellable context for the operation
	opCtx, cancel := context.WithCancel(ctx)

	// Register in-flight operation for cancellation support
	op := &inFlightOperation{
		requestID: requestID,
		subtype:   subtype,
		cancel:    cancel,
		startTime: time.Now(),
		completed: false,
	}

	c.inFlightMu.Lock()
	c.inFlight[requestID] = op
	c.inFlightMu.Unlock()

	// Run handler in goroutine so read loop can process cancel requests

	c.wg.Go(func() {
		// Cleanup: mark completed and remove from map
		defer func() {
			c.inFlightMu.Lock()
			defer c.inFlightMu.Unlock()

			op.completed = true

			delete(c.inFlight, requestID)

			cancel()
		}()

		// Invoke handler with cancellable context
		payload, err := handler(opCtx, req)

		// Check if cancelled
		if opCtx.Err() == context.Canceled {
			c.log.Debug("Handler was cancelled", "request_id", requestID)
			c.sendErrorResponse(ctx, requestID, errors.ErrOperationCancelled.Error())

			return
		}

		// Send response
		if err != nil {
			c.log.Warn("Handler returned error", "request_id", requestID, "error", err.Error())
			c.sendErrorResponse(ctx, requestID, err.Error())

			return
		}

		c.sendSuccessResponse(ctx, requestID, payload)
	})
}

// sendSuccessResponse sends a successful control response.
func (c *Controller) sendSuccessResponse(
	ctx context.Context,
	requestID string,
	payload map[string]any,
) {
	resp := &ControlResponse{
		Type: "control_response",
		Response: map[string]any{
			"subtype":    "success",
			"request_id": requestID,
			"response":   payload,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		c.log.Error("Failed to marshal control response", "error", err)

		return
	}

	if err := c.transport.SendMessage(ctx, data); err != nil {
		c.log.Error("Failed to send control response", "error", err)
	}
}

// sendErrorResponse sends an error control response.
func (c *Controller) sendErrorResponse(
	ctx context.Context,
	requestID string,
	errMsg string,
) {
	resp := &ControlResponse{
		Type: "control_response",
		Response: map[string]any{
			"subtype":    "error",
			"request_id": requestID,
			"error":      errMsg,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		c.log.Error("Failed to marshal error response", "error", err)

		return
	}

	if err := c.transport.SendMessage(ctx, data); err != nil {
		// Don't log error if context was cancelled (expected during shutdown)
		if ctx.Err() != nil {
			c.log.Debug("Could not send error response during shutdown", "error", err)

			return
		}

		c.log.Error("Failed to send error response", "error", err)
	}
}

// generateRequestID creates a unique request ID using ULID.
func (c *Controller) generateRequestID() string {
	return ulid.Make().String()
}

// handleCancelRequest handles control_cancel_request messages from the CLI.
// It looks up the in-flight operation and cancels its context if found.
func (c *Controller) handleCancelRequest(ctx context.Context, msg map[string]any) {
	requestID, ok := msg["request_id"].(string)
	if !ok {
		c.log.Warn("Cancel request missing request_id")

		return
	}

	c.log.Debug("Received cancel request", "request_id", requestID)

	c.inFlightMu.Lock()
	op, exists := c.inFlight[requestID]

	if !exists {
		c.inFlightMu.Unlock()
		c.log.Debug("Cancel request for unknown operation", "request_id", requestID)
		c.sendCancelAcknowledgment(ctx, requestID, false, false)

		return
	}

	alreadyCompleted := op.completed
	if !alreadyCompleted {
		op.cancel()
	}

	c.inFlightMu.Unlock()

	c.log.Debug("Cancel request processed",
		"request_id", requestID,
		"already_completed", alreadyCompleted,
	)

	c.sendCancelAcknowledgment(ctx, requestID, true, alreadyCompleted)
}

// sendCancelAcknowledgment sends a response acknowledging a cancel request.
func (c *Controller) sendCancelAcknowledgment(
	ctx context.Context,
	requestID string,
	found bool,
	alreadyCompleted bool,
) {
	resp := &ControlResponse{
		Type: "control_response",
		Response: map[string]any{
			"subtype":           "cancel_acknowledgment",
			"request_id":        requestID,
			"found":             found,
			"already_completed": alreadyCompleted,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		c.log.Error("Failed to marshal cancel acknowledgment", "error", err)

		return
	}

	if err := c.transport.SendMessage(ctx, data); err != nil {
		c.log.Error("Failed to send cancel acknowledgment", "error", err)
	}
}

// CancelAllInFlight cancels all in-flight operations.
// This is called during Stop() to ensure clean shutdown.
func (c *Controller) CancelAllInFlight() {
	c.inFlightMu.Lock()
	defer c.inFlightMu.Unlock()

	for _, op := range c.inFlight {
		if !op.completed {
			op.cancel()
		}
	}
}
