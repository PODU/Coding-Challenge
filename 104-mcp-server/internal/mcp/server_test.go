package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFraming_OneRequestOneResponse(t *testing.T) {
	srv, _ := newServer()
	resps := run(t, srv, initLine)
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	r := resps[0]
	if r.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", r.JSONRPC)
	}
	if string(r.ID) != "1" {
		t.Errorf("id = %s, want 1", r.ID)
	}
	if r.Error != nil {
		t.Errorf("unexpected error: %+v", r.Error)
	}
}

func TestFraming_MalformedJSONSurvives(t *testing.T) {
	srv, _ := newServer()
	resps := run(t, srv, `{not json`, initLine)
	if len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resps))
	}
	if resps[0].Error == nil || resps[0].Error.Code != CodeParseError {
		t.Fatalf("first response should be parse error, got %+v", resps[0])
	}
	if string(resps[0].ID) != "null" {
		t.Errorf("parse error id = %s, want null", resps[0].ID)
	}
	if resps[1].Error != nil {
		t.Errorf("loop did not recover: %+v", resps[1].Error)
	}
}

func TestFraming_NotificationNoResponse(t *testing.T) {
	srv, _ := newServer()
	// initialize (reply) + a bare notification (no reply).
	notif := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	resps := run(t, srv, initLine, notif)
	if len(resps) != 1 {
		t.Fatalf("notification produced a response: got %d responses", len(resps))
	}
}

func TestFraming_EOFCleanExit(t *testing.T) {
	srv, _ := newServer()
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(""), &out); err != nil {
		t.Fatalf("Serve on empty input = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("empty input produced output: %q", out.String())
	}
}

func TestLifecycle_CallBeforeInitialize(t *testing.T) {
	srv, _ := newServer()
	resps := run(t, srv, `{"jsonrpc":"2.0","id":5,"method":"tools/list"}`)
	if len(resps) != 1 || resps[0].Error == nil {
		t.Fatalf("expected an error response, got %+v", resps)
	}
	if resps[0].Error.Code != CodeNotInitialised {
		t.Errorf("code = %d, want %d", resps[0].Error.Code, CodeNotInitialised)
	}
}

func TestLifecycle_InitializeAdvertisesCapabilities(t *testing.T) {
	srv, _ := newServer()
	resps := run(t, srv, initLine)
	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools     *json.RawMessage `json:"tools"`
			Resources *json.RawMessage `json:"resources"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resps[0].Result, &res); err != nil {
		t.Fatalf("decode initialize result: %v", err)
	}
	if res.ProtocolVersion != protocolVersion {
		t.Errorf("protocolVersion = %q, want %q", res.ProtocolVersion, protocolVersion)
	}
	if res.Capabilities.Tools == nil || res.Capabilities.Resources == nil {
		t.Errorf("capabilities missing tools/resources: %+v", res.Capabilities)
	}
	if res.ServerInfo.Name != serverName || res.ServerInfo.Version != serverVersion {
		t.Errorf("serverInfo = %+v", res.ServerInfo)
	}
}

func TestLifecycle_DoubleInitializeRejected(t *testing.T) {
	srv, _ := newServer()
	second := `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`
	resps := run(t, srv, initLine, second)
	if len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resps))
	}
	if resps[1].Error == nil || resps[1].Error.Code != CodeInvalidRequest {
		t.Errorf("second initialize = %+v, want invalid request", resps[1])
	}
}

func TestLifecycle_Ping(t *testing.T) {
	srv, _ := newServer()
	resps := run(t, srv, initLine, `{"jsonrpc":"2.0","id":3,"method":"ping"}`)
	r := resps[1]
	if r.Error != nil {
		t.Fatalf("ping error: %+v", r.Error)
	}
	if strings.TrimSpace(string(r.Result)) != "{}" {
		t.Errorf("ping result = %s, want {}", r.Result)
	}
}

func TestUnknownMethod(t *testing.T) {
	srv, _ := newServer()
	resps := run(t, srv, initLine, `{"jsonrpc":"2.0","id":9,"method":"does/notExist"}`)
	if resps[1].Error == nil || resps[1].Error.Code != CodeMethodNotFound {
		t.Errorf("unknown method = %+v, want -32601", resps[1])
	}
}

func TestResources_List(t *testing.T) {
	srv, _ := newServer()
	resps := run(t, srv, initLine, `{"jsonrpc":"2.0","id":4,"method":"resources/list"}`)
	var res struct {
		Resources []Resource `json:"resources"`
	}
	if err := json.Unmarshal(resps[1].Result, &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := map[string]bool{}
	for _, r := range res.Resources {
		got[r.URI] = true
	}
	if !got[readmeURI] || !got[infoURI] {
		t.Errorf("resources missing expected URIs: %+v", res.Resources)
	}
}

func TestResources_ReadReadme(t *testing.T) {
	srv, _ := newServer()
	req := `{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"ccshared://readme"}}`
	resps := run(t, srv, initLine, req)
	var res struct {
		Contents []ResourceContent `json:"contents"`
	}
	if err := json.Unmarshal(resps[1].Result, &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "Shared Solutions") {
		t.Errorf("readme contents unexpected: %+v", res.Contents)
	}
	if res.Contents[0].MimeType != "text/markdown" {
		t.Errorf("mimeType = %q", res.Contents[0].MimeType)
	}
}

func TestResources_ReadUnknownURI(t *testing.T) {
	srv, _ := newServer()
	req := `{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"ccshared://nope"}}`
	resps := run(t, srv, initLine, req)
	if resps[1].Error == nil || resps[1].Error.Code != CodeInvalidParams {
		t.Errorf("unknown URI = %+v, want -32602", resps[1])
	}
}
