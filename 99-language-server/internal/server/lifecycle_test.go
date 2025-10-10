package server

import (
	"encoding/json"
	"testing"

	"greetls/internal/jsonrpc"
	"greetls/internal/lsp"
)

// Initialize advertises the expected capabilities.
func TestInitializeCapabilities(t *testing.T) {
	in := encodeFrames(t, initialize(1))
	_, out := runWith(t, in)

	resp := responseFor(out, "1")
	if resp == nil || resp.Error != nil {
		t.Fatalf("want initialize result, got %+v", resp)
	}
	var res lsp.InitializeResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if res.Capabilities.TextDocumentSync != lsp.SyncFull {
		t.Errorf("textDocumentSync = %d, want %d", res.Capabilities.TextDocumentSync, lsp.SyncFull)
	}
	if !res.Capabilities.HoverProvider {
		t.Error("hoverProvider should be true")
	}
	if res.Capabilities.CompletionProvider == nil {
		t.Error("completionProvider should be advertised")
	}
}

// The full handshake initialize -> initialized -> shutdown -> exit returns
// exit code 0, and shutdown replies null.
func TestFullHandshake(t *testing.T) {
	in := encodeFrames(t,
		initialize(1),
		initialized(),
		jsonrpc.NewRequest(2, "shutdown", nil),
		jsonrpc.NewNotification("exit", nil),
	)
	code, out := runWith(t, in)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	resp := responseFor(out, "2")
	if resp == nil || resp.Error != nil {
		t.Fatalf("want shutdown response, got %+v", resp)
	}
	if string(resp.Result) != "null" {
		t.Fatalf("shutdown result = %s, want null", resp.Result)
	}
}

// Exit without a prior shutdown returns exit code 1.
func TestExitWithoutShutdown(t *testing.T) {
	in := encodeFrames(t,
		initialize(1),
		initialized(),
		jsonrpc.NewNotification("exit", nil),
	)
	code, _ := runWith(t, in)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// A request after shutdown returns -32600.
func TestRequestAfterShutdown(t *testing.T) {
	in := encodeFrames(t,
		initialize(1),
		initialized(),
		jsonrpc.NewRequest(2, "shutdown", nil),
		hoverReq(3, "file:///x.greet", 0, 0),
		jsonrpc.NewNotification("exit", nil),
	)
	_, out := runWith(t, in)

	resp := responseFor(out, "3")
	if resp == nil || resp.Error == nil || resp.Error.Code != jsonrpc.CodeInvalidRequest {
		t.Fatalf("want -32600 for post-shutdown request, got %+v", resp)
	}
}
