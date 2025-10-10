package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"greetls/internal/jsonrpc"
	"greetls/internal/lsp"
)

// encodeFrames writes msgs as Content-Length-framed bytes.
func encodeFrames(t *testing.T, msgs ...*jsonrpc.Message) []byte {
	t.Helper()
	var buf bytes.Buffer
	st := jsonrpc.NewStream(nil, &buf)
	for _, m := range msgs {
		if err := st.Write(m); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	return buf.Bytes()
}

// runWith boots a Server on the given input bytes, runs it to completion, and
// returns the exit code plus every framed message the server wrote.
func runWith(t *testing.T, input []byte) (int, []*jsonrpc.Message) {
	t.Helper()
	out := &bytes.Buffer{}
	code := New(bytes.NewReader(input), out).Run()

	rd := jsonrpc.NewStream(bytes.NewReader(out.Bytes()), nil)
	var msgs []*jsonrpc.Message
	for {
		m, err := rd.Read()
		if err != nil {
			break
		}
		msgs = append(msgs, m)
	}
	return code, msgs
}

func responseFor(msgs []*jsonrpc.Message, id string) *jsonrpc.Message {
	for _, m := range msgs {
		if m.IsResponse() && string(m.ID) == id {
			return m
		}
	}
	return nil
}

func notificationFor(msgs []*jsonrpc.Message, method string) *jsonrpc.Message {
	for _, m := range msgs {
		if m.IsNotification() && m.Method == method {
			return m
		}
	}
	return nil
}

func initialize(id int) *jsonrpc.Message {
	return jsonrpc.NewRequest(id, "initialize", lsp.InitializeParams{})
}

func initialized() *jsonrpc.Message {
	return jsonrpc.NewNotification("initialized", struct{}{})
}

func hoverReq(id int, uri string, line, char int) *jsonrpc.Message {
	return jsonrpc.NewRequest(id, "textDocument/hover", lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: line, Character: char},
	})
}

// An unknown request method returns -32601.
func TestUnknownRequestMethod(t *testing.T) {
	in := encodeFrames(t, initialize(1), initialized(), jsonrpc.NewRequest(9, "foo/bar", nil))
	_, out := runWith(t, in)

	resp := responseFor(out, "9")
	if resp == nil || resp.Error == nil {
		t.Fatalf("want error response for id 9, got %+v", resp)
	}
	if resp.Error.Code != jsonrpc.CodeMethodNotFound {
		t.Fatalf("code = %d, want %d", resp.Error.Code, jsonrpc.CodeMethodNotFound)
	}
}

// An unknown notification produces no response.
func TestUnknownNotificationIgnored(t *testing.T) {
	in := encodeFrames(t, initialize(1), initialized(), jsonrpc.NewNotification("$/somethingRandom", nil))
	_, out := runWith(t, in)

	// The only response should be the initialize result.
	responses := 0
	for _, m := range out {
		if m.IsResponse() {
			responses++
		}
	}
	if responses != 1 {
		t.Fatalf("want exactly 1 response (initialize), got %d", responses)
	}
}

// An invalid JSON body with a recoverable id gets a -32700 reply.
func TestInvalidJSONBody(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":42,"method":}`
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	_, out := runWith(t, []byte(frame))

	resp := responseFor(out, "42")
	if resp == nil || resp.Error == nil || resp.Error.Code != jsonrpc.CodeParseError {
		t.Fatalf("want -32700 response for id 42, got %+v", resp)
	}
}

// A request before initialize returns -32002.
func TestRequestBeforeInitialize(t *testing.T) {
	in := encodeFrames(t, hoverReq(5, "file:///x.greet", 0, 0))
	_, out := runWith(t, in)

	resp := responseFor(out, "5")
	if resp == nil || resp.Error == nil || resp.Error.Code != jsonrpc.CodeServerNotInitialized {
		t.Fatalf("want -32002 response for id 5, got %+v", resp)
	}
}

// Full loop over pipes — open a bad doc and assert a publishDiagnostics
// notification is emitted with the expected diagnostics.
func TestEndToEndDiagnostics(t *testing.T) {
	const uri = "file:///bad.greet"
	badDoc := "greet\nrepeat foo\n" // bare greet (Error) + non-integer repeat (Error)

	in := encodeFrames(t,
		initialize(1),
		initialized(),
		jsonrpc.NewNotification("textDocument/didOpen", lsp.DidOpenTextDocumentParams{
			TextDocument: lsp.TextDocumentItem{URI: uri, LanguageID: "greet", Version: 1, Text: badDoc},
		}),
	)
	_, out := runWith(t, in)

	note := notificationFor(out, "textDocument/publishDiagnostics")
	if note == nil {
		t.Fatal("no publishDiagnostics notification emitted")
	}
	var p lsp.PublishDiagnosticsParams
	if err := json.Unmarshal(note.Params, &p); err != nil {
		t.Fatalf("decode diagnostics: %v", err)
	}
	if p.URI != uri {
		t.Errorf("uri = %q, want %q", p.URI, uri)
	}
	if len(p.Diagnostics) != 2 {
		t.Fatalf("want 2 diagnostics, got %d: %+v", len(p.Diagnostics), p.Diagnostics)
	}
	for _, d := range p.Diagnostics {
		if d.Severity != lsp.SeverityError {
			t.Errorf("expected Error severity, got %d (%q)", d.Severity, d.Message)
		}
	}
}

// didChange replaces document text and re-publishes diagnostics; opening a
// clean doc then breaking it shows the diagnostic appear.
func TestDidChangeRepublishes(t *testing.T) {
	const uri = "file:///doc.greet"
	in := encodeFrames(t,
		initialize(1),
		initialized(),
		jsonrpc.NewNotification("textDocument/didOpen", lsp.DidOpenTextDocumentParams{
			TextDocument: lsp.TextDocumentItem{URI: uri, Version: 1, Text: "greet \"Hi\"\n"},
		}),
		jsonrpc.NewNotification("textDocument/didChange", lsp.DidChangeTextDocumentParams{
			TextDocument:   lsp.VersionedTextDocumentIdentifier{URI: uri, Version: 2},
			ContentChanges: []lsp.TextDocumentContentChangeEvent{{Text: "greet\n"}},
		}),
	)
	_, out := runWith(t, in)

	var last *lsp.PublishDiagnosticsParams
	for _, m := range out {
		if m.IsNotification() && m.Method == "textDocument/publishDiagnostics" {
			var p lsp.PublishDiagnosticsParams
			if err := json.Unmarshal(m.Params, &p); err != nil {
				t.Fatal(err)
			}
			last = &p
		}
	}
	if last == nil {
		t.Fatal("no diagnostics published")
	}
	if last.Version != 2 {
		t.Errorf("last diagnostics version = %d, want 2", last.Version)
	}
	if len(last.Diagnostics) != 1 {
		t.Fatalf("after change want 1 diagnostic, got %+v", last.Diagnostics)
	}
}
