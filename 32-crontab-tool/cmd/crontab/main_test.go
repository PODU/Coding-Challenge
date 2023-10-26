package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCLIValidate(t *testing.T) {
	if code, out, _ := run(t, "validate", "0 9 * * SAT"); code != 0 || strings.TrimSpace(out) != "valid" {
		t.Errorf("validate good: code=%d out=%q", code, out)
	}
	if code, _, errOut := run(t, "validate", "99 * * * *"); code != 1 || !strings.Contains(errOut, "invalid") {
		t.Errorf("validate bad: code=%d err=%q", code, errOut)
	}
}

func TestCLIDescribe(t *testing.T) {
	code, out, _ := run(t, "describe", "0 9 * * SAT")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if strings.TrimSpace(out) != "At 09:00, only on Saturday" {
		t.Errorf("describe out=%q", out)
	}
}

func TestCLINext(t *testing.T) {
	code, out, _ := run(t, "next", "*/15 9-17 * * 1-5", "--from", "2026-06-26T08:50:00Z", "-n", "3")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	want := "2026-06-26T09:00:00Z\n2026-06-26T09:15:00Z\n2026-06-26T09:30:00Z\n"
	if out != want {
		t.Errorf("next out=%q, want %q", out, want)
	}
}

func TestCLIAddAndList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crontab")
	if code, out, errOut := run(t, "add", "0 2 * * *", "/usr/local/bin/backup.sh", "--file", path); code != 0 {
		t.Fatalf("add code=%d out=%q err=%q", code, out, errOut)
	}
	code, out, _ := run(t, "list", "--file", path)
	if code != 0 {
		t.Fatalf("list code=%d", code)
	}
	if !strings.Contains(out, "0 2 * * *") || !strings.Contains(out, "backup.sh") {
		t.Errorf("list out=%q", out)
	}
}

func TestCLIUsageErrors(t *testing.T) {
	if code, _, _ := run(t); code != 2 {
		t.Errorf("no args: code=%d, want 2", code)
	}
	if code, _, _ := run(t, "bogus"); code != 2 {
		t.Errorf("unknown command: code=%d, want 2", code)
	}
}
