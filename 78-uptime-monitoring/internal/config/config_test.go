package config

import (
	"strings"
	"testing"
	"time"

	"github.com/example/uptime/internal/monitor"
)

const validYAML = `
server:
  listen: ":9090"
storage:
  driver: memory
defaults:
  interval: 60s
  timeout: 10s
  method: HEAD
  expect_status: [200-399]
  alert_threshold: 3
alerters:
  - type: log
  - type: webhook
    url: https://hooks.example.com/x
monitors:
  - id: api
    name: API
    kind: http
    target: https://api.example.com/health
    interval: 30s
    timeout: 5s
    method: GET
    expect_status: [200, 201, 500-503]
    latency_threshold: 800ms
    alert_threshold: 2
  - id: db
    kind: tcp
    target: db.internal:5432
`

func TestParseValid(t *testing.T) {
	c, err := Parse([]byte(validYAML))
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Listen != ":9090" {
		t.Fatalf("listen = %q", c.Server.Listen)
	}
	monitors, err := c.BuildMonitors()
	if err != nil {
		t.Fatal(err)
	}
	if len(monitors) != 2 {
		t.Fatalf("got %d monitors", len(monitors))
	}

	api := monitors[0]
	if api.Method != "GET" || api.Kind != monitor.KindHTTP {
		t.Fatalf("api monitor = %+v", api)
	}
	if api.Interval != 30*time.Second || api.Timeout != 5*time.Second {
		t.Fatalf("api durations = %v/%v", api.Interval, api.Timeout)
	}
	if api.LatencyThreshold != 800*time.Millisecond {
		t.Fatalf("latency threshold = %v", api.LatencyThreshold)
	}
	if len(api.ExpectStatus) != 3 || !monitor.StatusMatches(api.ExpectStatus, 502) {
		t.Fatalf("expect_status = %+v", api.ExpectStatus)
	}

	// db monitor inherits defaults (interval 60s, method irrelevant for tcp).
	db := monitors[1]
	if db.Kind != monitor.KindTCP || db.Interval != 60*time.Second {
		t.Fatalf("db monitor = %+v", db)
	}
	if db.Name != "db" { // name defaults to id
		t.Fatalf("db name = %q", db.Name)
	}
	if db.AlertThreshold != 3 { // inherited default
		t.Fatalf("db alert threshold = %d", db.AlertThreshold)
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"duplicate id": `
monitors:
  - {id: a, kind: tcp, target: h:1, interval: 30s, timeout: 5s}
  - {id: a, kind: tcp, target: h:2, interval: 30s, timeout: 5s}
`,
		"timeout >= interval": `
monitors:
  - {id: a, kind: http, target: "https://x/", interval: 5s, timeout: 5s}
`,
		"bad http target": `
monitors:
  - {id: a, kind: http, target: "not-a-url", interval: 30s, timeout: 5s}
`,
		"bad tcp target": `
monitors:
  - {id: a, kind: tcp, target: "no-port", interval: 30s, timeout: 5s}
`,
		"bad kind": `
monitors:
  - {id: a, kind: smtp, target: "h:25", interval: 30s, timeout: 5s}
`,
		"unknown key": `
monitors:
  - {id: a, kind: tcp, target: "h:1", interval: 30s, timeout: 5s, bogus: 1}
`,
		"empty id": `
monitors:
  - {kind: tcp, target: "h:1", interval: 30s, timeout: 5s}
`,
		"bad method": `
monitors:
  - {id: a, kind: http, target: "https://x/", interval: 30s, timeout: 5s, method: PUT}
`,
	}
	for name, yml := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(yml)); err == nil {
				t.Fatalf("expected error for %q, got nil", name)
			}
		})
	}
}

func TestParseAppliesDefaultsWhenOmitted(t *testing.T) {
	const yml = `
monitors:
  - {id: a, kind: http, target: "https://x/"}
`
	c, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := c.BuildMonitors()
	if m[0].Interval != 60*time.Second || m[0].Timeout != 10*time.Second {
		t.Fatalf("defaults not applied: %+v", m[0])
	}
	if m[0].Method != "HEAD" {
		t.Fatalf("default method = %q", m[0].Method)
	}
	if !strings.EqualFold(m[0].Kind.String(), "http") {
		t.Fatalf("kind = %v", m[0].Kind)
	}
}
