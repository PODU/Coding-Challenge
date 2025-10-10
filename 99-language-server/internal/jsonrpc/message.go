// Package jsonrpc reads and writes JSON-RPC 2.0 messages with LSP-style
// Content-Length framing. It knows nothing about LSP methods.
package jsonrpc

import (
	"encoding/json"
	"fmt"
)

// Version is the JSON-RPC protocol version string.
const Version = "2.0"

// Standard JSON-RPC and LSP error codes.
const (
	CodeParseError           = -32700
	CodeInvalidRequest       = -32600
	CodeMethodNotFound       = -32601
	CodeInvalidParams        = -32602
	CodeInternalError        = -32603
	CodeServerNotInitialized = -32002
)

// Message is a request (ID and Method), a notification (Method only) or a
// response (ID only). ID stays raw so a numeric or string id is echoed back
// exactly as received.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// IsRequest reports whether m is a request (has both id and method).
func (m *Message) IsRequest() bool { return len(m.ID) != 0 && m.Method != "" }

// IsNotification reports whether m is a notification (method, no id).
func (m *Message) IsNotification() bool { return len(m.ID) == 0 && m.Method != "" }

// IsResponse reports whether m is a response (id, no method).
func (m *Message) IsResponse() bool { return len(m.ID) != 0 && m.Method == "" }

// Error is a JSON-RPC error object.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message) }

// FramingError means a bad header block. The stream is still usable after it.
type FramingError struct{ Reason string }

func (e *FramingError) Error() string { return "framing error: " + e.Reason }

// ParseError means the body was not valid JSON. ID is whatever request id
// could be scraped from the raw bytes, so the caller can still reply.
type ParseError struct {
	ID  json.RawMessage
	Err error
}

func (e *ParseError) Error() string { return "parse error: " + e.Err.Error() }

// NewRequest builds a request. id may be a number or a string; nil params are
// omitted.
func NewRequest(id interface{}, method string, params interface{}) *Message {
	return &Message{JSONRPC: Version, ID: mustRaw(id), Method: method, Params: mustRaw(params)}
}

// NewNotification builds a notification message (no id).
func NewNotification(method string, params interface{}) *Message {
	return &Message{JSONRPC: Version, Method: method, Params: mustRaw(params)}
}

func mustRaw(v interface{}) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
