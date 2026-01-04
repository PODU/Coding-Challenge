package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolsList(t *testing.T) {
	srv, _ := newServer()
	resps := run(t, srv, initLine, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	var res struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(resps[1].Result, &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := map[string]Tool{}
	for _, tl := range res.Tools {
		got[tl.Name] = tl
	}
	for _, name := range []string{"hello", "CodingChallengesSolutionFinder"} {
		tl, ok := got[name]
		if !ok {
			t.Fatalf("tools/list missing %q", name)
		}
		// inputSchema must be a JSON object.
		var schema map[string]any
		if err := json.Unmarshal(tl.InputSchema, &schema); err != nil {
			t.Errorf("%s inputSchema not an object: %v", name, err)
		}
		if schema["type"] != "object" {
			t.Errorf("%s inputSchema type = %v", name, schema["type"])
		}
	}
}

func TestHello_OK(t *testing.T) {
	srv, _ := newServer()
	call := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"hello","arguments":{"name":"John"}}}`
	resps := run(t, srv, initLine, call)
	res := callToolResult(t, resps[1])
	if res.IsError {
		t.Fatalf("isError true: %+v", res)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "Hello, John" {
		t.Errorf("content = %+v, want 'Hello, John'", res.Content)
	}
}

func TestHello_MissingName(t *testing.T) {
	srv, _ := newServer()
	call := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"hello","arguments":{}}}`
	resps := run(t, srv, initLine, call)
	if resps[1].Error == nil || resps[1].Error.Code != CodeInvalidParams {
		t.Errorf("missing name = %+v, want -32602", resps[1])
	}
}

func TestToolsCall_UnknownTool(t *testing.T) {
	srv, _ := newServer()
	call := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"nope","arguments":{}}}`
	resps := run(t, srv, initLine, call)
	if resps[1].Error == nil || resps[1].Error.Code != CodeInvalidParams {
		t.Errorf("unknown tool = %+v, want -32602", resps[1])
	}
}

func TestFinder_QueryMatches(t *testing.T) {
	srv, _ := newServer()
	call := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"CodingChallengesSolutionFinder","arguments":{"query":"wc"}}}`
	resps := run(t, srv, initLine, call)
	res := callToolResult(t, resps[1])
	if res.IsError {
		t.Fatalf("unexpected isError: %+v", res)
	}
	text := res.Content[0].Text
	for _, want := range []string{"https://example.com/wc/go", "https://example.com/wc/python", "https://example.com/wc/rust"} {
		if !strings.Contains(text, want) {
			t.Errorf("result missing %q\n%s", want, text)
		}
	}
}

func TestFinder_NoMatch(t *testing.T) {
	srv, _ := newServer()
	call := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"CodingChallengesSolutionFinder","arguments":{"query":"doesnotexist"}}}`
	resps := run(t, srv, initLine, call)
	res := callToolResult(t, resps[1])
	if res.IsError {
		t.Errorf("empty result should not be an error: %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "No matching challenges") {
		t.Errorf("text = %q", res.Content[0].Text)
	}
}

func TestFinder_LanguageFilter(t *testing.T) {
	srv, _ := newServer()
	call := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"CodingChallengesSolutionFinder","arguments":{"query":"wc","language":"Go"}}}`
	resps := run(t, srv, initLine, call)
	res := callToolResult(t, resps[1])
	text := res.Content[0].Text
	if !strings.Contains(text, "https://example.com/wc/go") {
		t.Errorf("missing go link: %s", text)
	}
	if strings.Contains(text, "wc/python") || strings.Contains(text, "wc/rust") {
		t.Errorf("language filter leaked other languages: %s", text)
	}
}

func TestFinder_LanguageNoMatch(t *testing.T) {
	srv, _ := newServer()
	call := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"CodingChallengesSolutionFinder","arguments":{"query":"wc","language":"Haskell"}}}`
	resps := run(t, srv, initLine, call)
	res := callToolResult(t, resps[1])
	if res.IsError {
		t.Errorf("should not be error: %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "Haskell") {
		t.Errorf("expected message naming the language, got %q", res.Content[0].Text)
	}
}

func TestFinder_FetcherError(t *testing.T) {
	f := &fakeFetcher{err: errFetchBoom}
	srv := NewServer(f)
	call := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"CodingChallengesSolutionFinder","arguments":{"query":"wc"}}}`
	// Also send a follow-up ping to prove the server stays responsive.
	resps := run(t, srv, initLine, call, `{"jsonrpc":"2.0","id":9,"method":"ping"}`)
	res := callToolResult(t, resps[1])
	if !res.IsError {
		t.Fatalf("fetch failure should set isError, got %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "Failed to fetch") {
		t.Errorf("text = %q", res.Content[0].Text)
	}
	if resps[2].Error != nil {
		t.Errorf("server not responsive after fetch error: %+v", resps[2])
	}
}

func TestFinder_FormatChanged(t *testing.T) {
	f := &fakeFetcher{readme: "# Title\n\nJust prose, no solution links at all.\n"}
	srv := NewServer(f)
	call := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"CodingChallengesSolutionFinder","arguments":{"query":"wc"}}}`
	resps := run(t, srv, initLine, call)
	res := callToolResult(t, resps[1])
	if !res.IsError {
		t.Fatalf("no parseable links should set isError, got %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "format may have changed") {
		t.Errorf("text = %q", res.Content[0].Text)
	}
}
