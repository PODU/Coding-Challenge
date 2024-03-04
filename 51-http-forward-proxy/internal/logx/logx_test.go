package logx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccessWritesToExtraWriter(t *testing.T) {
	var buf bytes.Buffer
	lg := New(&buf)
	lg.Access("203.0.113.7", "http://example.org/page", 200)

	out := buf.String()
	if !strings.Contains(out, "ACCESS") {
		t.Errorf("missing ACCESS tag: %q", out)
	}
	if !strings.Contains(out, "client=203.0.113.7") {
		t.Errorf("missing client IP: %q", out)
	}
	if !strings.Contains(out, `url="http://example.org/page"`) {
		t.Errorf("missing url: %q", out)
	}
	if !strings.Contains(out, "status=200") {
		t.Errorf("missing status: %q", out)
	}
	// Timestamp prefix from log.LstdFlags: "YYYY/MM/DD".
	if !strings.HasPrefix(out, "20") {
		t.Errorf("expected timestamp prefix, got %q", out)
	}
}

func TestNewWithFileWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.log")
	lg, err := NewWithFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lg.Access("198.51.100.1", "http://x.test/", 403)
	lg.Tunnel("198.51.100.1", "x.test:443", "open")
	if err := lg.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "status=403") {
		t.Errorf("file missing access line: %q", content)
	}
	if !strings.Contains(content, "CONNECT") {
		t.Errorf("file missing tunnel line: %q", content)
	}
}
