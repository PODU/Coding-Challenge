package mcp

import "encoding/json"

// JSON-RPC error codes. -32002 is MCP's "server not initialised"; the rest
// are standard JSON-RPC codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	CodeNotInitialised = -32002
)

// Request is an incoming JSON-RPC 2.0 message. A message with no ID is a
// notification and must not receive a response.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification reports whether the request has no ID.
func (r *Request) IsNotification() bool { return len(r.ID) == 0 }

// Response is an outgoing JSON-RPC 2.0 message. Exactly one of Result or
// Error is set.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// idOrNull returns id, or JSON null if there is none (e.g. after a parse
// error).
func idOrNull(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}

func resultResponse(id json.RawMessage, result any) *Response {
	b, err := json.Marshal(result)
	if err != nil {
		return errorResponse(id, CodeInternalError, "failed to marshal result: "+err.Error())
	}
	return &Response{JSONRPC: "2.0", ID: idOrNull(id), Result: b}
}

func errorResponse(id json.RawMessage, code int, msg string) *Response {
	return &Response{JSONRPC: "2.0", ID: idOrNull(id), Error: &RPCError{Code: code, Message: msg}}
}

// paramError means the tool arguments were missing or malformed. It becomes a
// -32602 JSON-RPC error rather than an in-band CallToolResult.IsError.
type paramError struct{ msg string }

func (e *paramError) Error() string { return e.msg }
