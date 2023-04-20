package config

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFlagsOnly(t *testing.T) {
	cfg, err := Load([]string{
		"-listen", ":9000",
		"-backend", "http://a:8080",
		"-backend", "http://b:8080",
		"-health-interval", "5s",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":9000" {
		t.Errorf("listen = %q, want :9000", cfg.Listen)
	}
	if len(cfg.Backends) != 2 {
		t.Errorf("backends = %v, want 2", cfg.Backends)
	}
	if cfg.HealthInterval != 5*time.Second {
		t.Errorf("interval = %v, want 5s", cfg.HealthInterval)
	}
	// Untouched flag keeps its default.
	if cfg.HealthPath != DefaultHealthPath {
		t.Errorf("health path = %q, want default", cfg.HealthPath)
	}
}

func writeYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "lb.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFileOnly(t *testing.T) {
	path := writeYAML(t, `
listen: ":7000"
backends:
  - http://x:8080
  - http://y:8080
health:
  path: "/healthz"
  interval: "3s"
  timeout: "1s"
strategy: "round_robin"
`)
	cfg, err := Load([]string{"-config", path}, io.Discard)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":7000" {
		t.Errorf("listen = %q, want :7000", cfg.Listen)
	}
	if cfg.HealthPath != "/healthz" {
		t.Errorf("health path = %q, want /healthz", cfg.HealthPath)
	}
	if cfg.HealthInterval != 3*time.Second {
		t.Errorf("interval = %v, want 3s", cfg.HealthInterval)
	}
	if cfg.HealthTimeout != time.Second {
		t.Errorf("timeout = %v, want 1s", cfg.HealthTimeout)
	}
	if len(cfg.Backends) != 2 {
		t.Errorf("backends = %v, want 2", cfg.Backends)
	}
}

func TestFlagsOverrideFile(t *testing.T) {
	path := writeYAML(t, `
listen: ":7000"
backends:
  - http://x:8080
health:
  interval: "3s"
`)
	cfg, err := Load([]string{
		"-config", path,
		"-listen", ":9999",
		"-health-interval", "20s",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":9999" {
		t.Errorf("flag should override file listen: got %q", cfg.Listen)
	}
	if cfg.HealthInterval != 20*time.Second {
		t.Errorf("flag should override file interval: got %v", cfg.HealthInterval)
	}
	// Backend came only from the file.
	if len(cfg.Backends) != 1 || cfg.Backends[0] != "http://x:8080" {
		t.Errorf("backends = %v, want [http://x:8080]", cfg.Backends)
	}
}

func TestValidationFailures(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no backends", []string{"-listen", ":9000"}},
		{"bad url", []string{"-backend", "://nope"}},
		{"scheme missing", []string{"-backend", "localhost:8080"}},
		{"duplicate", []string{"-backend", "http://a:8080", "-backend", "http://a:8080"}},
		{"bad strategy", []string{"-backend", "http://a:8080", "-strategy", "magic"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(tc.args, io.Discard); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestNonPositiveIntervalRejected(t *testing.T) {
	path := writeYAML(t, `
backends:
  - http://a:8080
health:
  interval: "0s"
`)
	if _, err := Load([]string{"-config", path}, io.Discard); err == nil {
		t.Fatal("expected error for non-positive interval")
	}
}
