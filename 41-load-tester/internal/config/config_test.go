package config

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func noFile(string) ([]byte, error) { return nil, errors.New("no file") }

func TestParseDefaults(t *testing.T) {
	cfg, err := parse([]string{"-u", "http://example.com"}, noFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.N != 10 {
		t.Errorf("N = %d, want default 10", cfg.N)
	}
	if cfg.C != 1 {
		t.Errorf("C = %d, want default 1", cfg.C)
	}
	if cfg.Method != "GET" {
		t.Errorf("Method = %q, want GET", cfg.Method)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestParseValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no source", []string{"-n", "5"}, "required"},
		{"both sources", []string{"-u", "http://x.com", "-f", "list.txt"}, "mutually exclusive"},
		{"n zero", []string{"-u", "http://x.com", "-n", "0"}, "-n must be >= 1"},
		{"c zero", []string{"-u", "http://x.com", "-c", "0"}, "-c must be >= 1"},
		{"bad method", []string{"-u", "http://x.com", "-m", "FETCH"}, "unsupported HTTP method"},
		{"bad scheme", []string{"-u", "ftp://x.com"}, "invalid URL scheme"},
		{"no host", []string{"-u", "http://"}, "missing host"},
		{"bad header", []string{"-u", "http://x.com", "-H", "NoColonHere"}, "malformed header"},
		{"empty header key", []string{"-u", "http://x.com", "-H", ": value"}, "malformed header"},
		{"bad timeout", []string{"-u", "http://x.com", "-t", "0s"}, "-t must be > 0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parse(c.args, noFile)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want substring %q", err.Error(), c.want)
			}
		})
	}
}

func TestParseConcurrencyClamp(t *testing.T) {
	cfg, err := parse([]string{"-u", "http://x.com", "-n", "5", "-c", "20"}, noFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.C != 5 {
		t.Errorf("C = %d, want clamped to 5", cfg.C)
	}
	if len(cfg.Warnings) == 0 {
		t.Errorf("expected a clamp warning, got none")
	}
}

func TestParseHeaders(t *testing.T) {
	cfg, err := parse([]string{
		"-u", "http://x.com",
		"-H", "Authorization: Bearer abc",
		"-H", "X-Trace: 123",
	}, noFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Headers.Get("Authorization"); got != "Bearer abc" {
		t.Errorf("Authorization = %q, want 'Bearer abc'", got)
	}
	if got := cfg.Headers.Get("X-Trace"); got != "123" {
		t.Errorf("X-Trace = %q, want '123'", got)
	}
}

func TestParseBodyFromFile(t *testing.T) {
	fake := func(path string) ([]byte, error) {
		if path != "payload.json" {
			t.Fatalf("unexpected path %q", path)
		}
		return []byte(`{"hello":"world"}`), nil
	}
	cfg, err := parse([]string{"-u", "http://x.com", "-d", "@payload.json"}, fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Body != `{"hello":"world"}` {
		t.Errorf("Body = %q, want loaded file content", cfg.Body)
	}
}

func TestParseBodyLiteral(t *testing.T) {
	cfg, err := parse([]string{"-u", "http://x.com", "-d", "raw=body"}, noFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Body != "raw=body" {
		t.Errorf("Body = %q, want literal", cfg.Body)
	}
}
