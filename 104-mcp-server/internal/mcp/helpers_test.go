package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fakeFetcher is an offline stand-in for GitHubFetcher.
type fakeFetcher struct {
	readme string
	err    error
	calls  int
}

func (f *fakeFetcher) Fetch(_ context.Context) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.readme, nil
}

// fixtureReadme is a small, well-formed README used across tests.
const fixtureReadme = `# Coding Challenges — Shared Solutions

Community solutions to John Crickett's Coding Challenges.

## Challenges

### Challenge 1 - Write Your Own wc Tool
- [Go](https://example.com/wc/go)
- [Python](https://example.com/wc/python)
- [Rust](https://example.com/wc/rust)

### Challenge 2 - Write Your Own JSON Parser
- [Go](https://example.com/json/go)
- [JavaScript](https://example.com/json/js)

### Challenge 104 - Build Your Own MCP Server
- [Go](https://example.com/mcp/go)
`

// run drives Server.Serve with the given request lines and decodes every
// response line. Notifications produce no line, so the count may be smaller
// than the number of requests.
func run(t *testing.T, srv *Server, requests ...string) []Response {
	t.Helper()
	in := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var out bytes.Buffer
	if err := srv.Serve(in, &out); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
	var resps []Response
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r Response
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("response line is not valid JSON %q: %v", line, err)
		}
		resps = append(resps, r)
	}
	return resps
}

// initLine is a ready-made initialize request used to satisfy the lifecycle.
const initLine = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`

// newServer builds a server backed by the fixture README.
func newServer() (*Server, *fakeFetcher) {
	f := &fakeFetcher{readme: fixtureReadme}
	return NewServer(f), f
}

// callToolResult decodes a tools/call result.
func callToolResult(t *testing.T, r Response) CallToolResult {
	t.Helper()
	if r.Error != nil {
		t.Fatalf("expected result, got error %+v", r.Error)
	}
	var res CallToolResult
	if err := json.Unmarshal(r.Result, &res); err != nil {
		t.Fatalf("decode CallToolResult: %v", err)
	}
	return res
}

// errIs sanity-checks an error value used by fakeFetcher.
var errFetchBoom = errors.New("network down")
