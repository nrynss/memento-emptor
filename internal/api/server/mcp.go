package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nrynss/keel/wire"
	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/auth"
	"github.com/nrynss/memento-emptor/internal/ingest"
	"github.com/nrynss/memento-emptor/internal/version"
)

// loadSchema reads one frozen request schema for tools/list. The schemas
// are the contract's: the service describes exactly what it serves.
func loadSchema(dir, name string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("server: no schema dir for %s", name)
	}
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", fmt.Errorf("server: load schema %s: %w", name, err)
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("server: schema %s is not JSON", name)
	}
	return string(raw), nil
}

// Handler returns the service's HTTP handler: MCP over JSON-RPC, one
// plain endpoint per tool for the ledger, the call log, webhook ingest
// and a health check. Errors on plain routes use the shared error
// envelope; MCP tool errors ride inside the result.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp", s.handleMCP)
	mux.HandleFunc("POST /tools/{tool}", s.handleTool)
	mux.HandleFunc("GET /calls", s.handleCalls)
	mux.HandleFunc("POST /webhooks/payment", s.handleWebhook)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	return mux
}

// handleHealth answers the deployment check with no state.
func (s *Service) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleWebhook ingests one raw payment-source delivery. The body is the
// exact bytes the source posted, bounded like every other route: the
// signature covers those bytes, so they are never re-encoded. The
// delivery's custom id is the shopper join — the reference agent sets it
// to the shopper at order creation — and a delivery without one is
// refused. Verification runs inside HandleDelivery before the
// idempotency check, so a forged body naming a stored event id is
// refused, never answered as a duplicate.
func (s *Service) handleWebhook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireError(w, http.StatusBadRequest, wire.CodeInvalidRequest, "unreadable body")
		return
	}
	shopper, err := webhookShopper(raw)
	if err != nil {
		writeWireError(w, http.StatusBadRequest, wire.CodeInvalidRequest, err.Error())
		return
	}
	header := r.Header.Clone()
	header.Del("Authorization")
	res, err := s.IngestDelivery(r.Context(), shopper, ingest.Delivery{Header: header, Body: raw})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// webhookShopper resolves the shopper one delivery belongs to from the
// body's resource custom id. The body's shape is the payment source's:
// unknown fields are the source's business and never fail the join.
func webhookShopper(raw []byte) (string, error) {
	var body struct {
		Resource struct {
			CustomID string `json:"custom_id"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", fmt.Errorf("bad delivery: %w", err)
	}
	if strings.TrimSpace(body.Resource.CustomID) == "" {
		return "", errors.New("delivery names no shopper")
	}
	return body.Resource.CustomID, nil
}

// callsResponse wraps the call log for the ledger.
type callsResponse struct {
	// Calls are the logged calls in arrival order.
	Calls []CallEntry `json:"calls"`
}

// callsFilter narrows the agent-only call log to one shopper when the
// edge scoped the request: the X-Memento-Shopper header carries the
// shopper the guard checked, and the layer trusts it without re-reading
// Authorization.
func callsFilter(calls []CallEntry, shopper string) []CallEntry {
	if shopper == "" {
		return calls
	}
	out := make([]CallEntry, 0, len(calls))
	for _, entry := range calls {
		if entry.ShopperID == shopper {
			out = append(out, entry)
		}
	}
	return out
}

// handleCalls serves the call log, optionally narrowed to one host trace
// so the ledger can show that a recommendation followed a recall. The log
// is agent-only: it carries every shopper's entries and the served layer
// does not scope it per shopper — the edge admits only agent keys. When
// the guard scoped the request to a shopper it says so in the
// X-Memento-Shopper header, and the layer honours it; the layer never
// re-reads Authorization, which the guard already removed.
func (s *Service) handleCalls(w http.ResponseWriter, r *http.Request) {
	calls := callsFilter(s.Calls(r.URL.Query().Get("trace_id")), r.Header.Get(auth.HeaderShopper))
	writeJSON(w, http.StatusOK, callsResponse{Calls: calls})
}

// handleTool serves one frozen tool over plain HTTP for clients without
// an MCP transport. The body is the tool's request; the response is the
// tool's response.
func (s *Service) handleTool(w http.ResponseWriter, r *http.Request) {
	tool := r.PathValue("tool")
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireError(w, http.StatusBadRequest, wire.CodeInvalidRequest, "unreadable body")
		return
	}
	out, err := s.dispatch(r.Context(), tool, raw)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// writeServiceError maps the service's sentinel conditions to statuses.
// Anything else is a dependency fault.
func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := wire.CodeInternal
	switch {
	case isNotFound(err):
		status, code = http.StatusNotFound, wire.CodeNotFound
	case isInvalid(err):
		status, code = http.StatusBadRequest, wire.CodeInvalidRequest
	}
	writeWireError(w, status, code, messageOf(err))
}

// messageOf reports the outermost message without internal wrapping.
func messageOf(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// isNotFound reports the absence conditions.
func isNotFound(err error) bool {
	return errors.Is(err, ErrUnknownBelief) || errors.Is(err, ErrUnknownQuestion) || errors.Is(err, ErrUnknownPurchase)
}

// isInvalid reports the refused-value conditions. A delivery that fails
// verification is the caller's body, not the layer's fault.
func isInvalid(err error) bool {
	return errors.Is(err, ErrQuestionClosed) || errors.Is(err, ErrFeedbackMismatch) ||
		errors.Is(err, ErrFeedbackConflict) ||
		errors.Is(err, ErrEmptyCorrection) || errors.Is(err, ErrMissingShopper) || errors.Is(err, ErrAlreadySeeded) ||
		errors.Is(err, ingest.ErrBadSignature) || errors.Is(err, ingest.ErrMalformedEvent)
}

// writeWireError writes the shared error envelope.
func writeWireError(w http.ResponseWriter, status int, code, message string) {
	_ = wire.WriteError(w, status, code, message, nil)
}

// writeJSON writes one JSON response.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// decodeBody reads one bounded JSON body with unknown fields rejected,
// so drift between caller and contract fails fast.
func decodeBody(r *http.Request, target any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("unreadable body: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("bad request: %w", err)
	}
	return nil
}

// jsonRPCRequest is one JSON-RPC 2.0 request.
type jsonRPCRequest struct {
	// JSONRPC carries the protocol version, always "2.0".
	JSONRPC string `json:"jsonrpc"`
	// ID echoes in the response. A missing id is a notification.
	ID *json.RawMessage `json:"id"`
	// Method names the MCP method.
	Method string `json:"method"`
	// Params carries the method's arguments.
	Params json.RawMessage `json:"params"`
}

// jsonRPCResponse is one JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	// JSONRPC carries the protocol version, always "2.0".
	JSONRPC string `json:"jsonrpc"`
	// ID echoes the request id, or null for errors before one parsed.
	ID *json.RawMessage `json:"id"`
	// Result carries the success value.
	Result any `json:"result,omitempty"`
	// Error carries the failure.
	Error *jsonRPCError `json:"error,omitempty"`
}

// jsonRPCError is one JSON-RPC 2.0 error.
type jsonRPCError struct {
	// Code is the JSON-RPC error code.
	Code int `json:"code"`
	// Message states the failure in words.
	Message string `json:"message"`
}

// mcpToolCall carries a tools/call's arguments.
type mcpToolCall struct {
	// Name is the frozen tool name.
	Name string `json:"name"`
	// Arguments is the tool's request object.
	Arguments json.RawMessage `json:"arguments"`
}

// mcpContent is one MCP result content block.
type mcpContent struct {
	// Type is always "text".
	Type string `json:"type"`
	// Text carries the tool's response JSON.
	Text string `json:"text"`
}

// mcpToolResult is a tools/call success value.
type mcpToolResult struct {
	// Content carries the response blocks.
	Content []mcpContent `json:"content"`
	// IsError marks a tool-level failure.
	IsError bool `json:"isError,omitempty"`
}

// mcpToolDescription is one tools/list entry.
type mcpToolDescription struct {
	// Name is the frozen tool name.
	Name string `json:"name"`
	// Description is the short text an agent reads before calling.
	Description string `json:"description"`
	// InputSchema is the tool's frozen request schema.
	InputSchema json.RawMessage `json:"inputSchema"`
}

// handleMCP serves the MCP surface: initialize, tools/list and
// tools/call over JSON-RPC 2.0. Tool-level failures ride inside the
// result with isError; protocol failures use JSON-RPC errors.
func (s *Service) handleMCP(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeRPC(w, nil, nil, rpcError(-32700, "unreadable body"))
		return
	}
	var req jsonRPCRequest
	if err := decodeSingle(raw, &req, "mcp"); err != nil {
		writeRPC(w, nil, nil, rpcError(-32700, "bad request"))
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		writeRPC(w, req.ID, nil, rpcError(-32600, "invalid request"))
		return
	}
	switch req.Method {
	case "initialize":
		writeRPC(w, req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "memento-emptor", "version": version.Version},
		}, nil)
	case "notifications/initialized":
		w.WriteHeader(http.StatusNoContent)
	case "tools/list":
		writeRPC(w, req.ID, map[string]any{"tools": s.describeTools()}, nil)
	case "tools/call":
		var call mcpToolCall
		if err := decodeSingle(req.Params, &call, "tools/call"); err != nil || call.Name == "" {
			writeRPC(w, req.ID, nil, rpcError(-32602, "bad tools/call params"))
			return
		}
		args := call.Arguments
		if len(args) == 0 {
			args = json.RawMessage("{}")
		}
		out, err := s.dispatch(r.Context(), call.Name, args)
		if err != nil {
			writeRPC(w, req.ID, mcpToolResult{
				Content: []mcpContent{{Type: "text", Text: fmt.Sprintf(`{"error":%q}`, messageOf(err))}},
				IsError: true,
			}, nil)
			return
		}
		writeRPC(w, req.ID, mcpToolResult{Content: []mcpContent{{Type: "text", Text: string(out)}}}, nil)
	default:
		writeRPC(w, req.ID, nil, rpcError(-32601, "unknown method "+req.Method))
	}
}

// describeTools lists the eleven frozen tools with their request schemas.
func (s *Service) describeTools() []mcpToolDescription {
	out := []mcpToolDescription{}
	for _, tool := range contract.Tools() {
		out = append(out, mcpToolDescription{
			Name:        string(tool.Name),
			Description: tool.Description,
			InputSchema: json.RawMessage(s.schemas[string(tool.Name)]),
		})
	}
	return out
}

// writeRPC writes one JSON-RPC response.
func writeRPC(w http.ResponseWriter, id *json.RawMessage, result any, rpcErr *jsonRPCError) {
	writeJSON(w, http.StatusOK, jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr})
}

// rpcError builds one JSON-RPC error.
func rpcError(code int, message string) *jsonRPCError {
	return &jsonRPCError{Code: code, Message: message}
}
