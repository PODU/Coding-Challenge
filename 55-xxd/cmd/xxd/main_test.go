package main

import (
	"bytes"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	var out, err bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &err)
	return code, out.String(), err.String()
}

func TestCLIStdinDump(t *testing.T) {
	code, out, _ := runCLI(t, nil, "File 1 contents.\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "00000000: 4669 6c65 2031 2063 6f6e 7465 6e74 732e  File 1 contents.\n" +
		"00000010: 0a                                       .\n"
	if out != want {
		t.Errorf("stdin dump:\n got %q\nwant %q", out, want)
	}
}

func TestCLIRoundTripPlain(t *testing.T) {
	code, dump, _ := runCLI(t, []string{"-p"}, "hello world")
	if code != 0 {
		t.Fatalf("dump exit %d", code)
	}
	code, restored, _ := runCLI(t, []string{"-r", "-p"}, dump)
	if code != 0 {
		t.Fatalf("reverse exit %d", code)
	}
	if restored != "hello world" {
		t.Errorf("plain round-trip: got %q", restored)
	}
}

func TestCLIHexSeekValue(t *testing.T) {
	// -s accepts 0x-prefixed hex.
	code, out, errOut := runCLI(t, []string{"-s", "0x4", "-l", "0x3"}, "File 1 contents.\n")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	want := "00000004: 2031 20                                   1 \n"
	if out != want {
		t.Errorf("hex seek:\n got %q\nwant %q", out, want)
	}
}

func TestCLIUnknownFlag(t *testing.T) {
	code, _, errOut := runCLI(t, []string{"-z"}, "")
	if code == 0 {
		t.Error("expected non-zero exit for unknown flag")
	}
	if !strings.Contains(errOut, "unknown option") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCLIBadGroupEndian(t *testing.T) {
	code, _, errOut := runCLI(t, []string{"-e", "-g", "3"}, "x")
	if code == 0 {
		t.Error("expected non-zero exit for -e with non-power-of-two group")
	}
	if !strings.Contains(errOut, "power of 2") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCLINegativeLength(t *testing.T) {
	code, _, _ := runCLI(t, []string{"-l", "-5"}, "x")
	if code == 0 {
		t.Error("expected non-zero exit for negative length")
	}
}

func TestCLIHelpVersion(t *testing.T) {
	if code, out, _ := runCLI(t, []string{"-h"}, ""); code != 0 || !strings.Contains(out, "Usage:") {
		t.Errorf("help: code=%d out=%q", code, out)
	}
	if code, out, _ := runCLI(t, []string{"-v"}, ""); code != 0 || !strings.Contains(out, "xxd") {
		t.Errorf("version: code=%d out=%q", code, out)
	}
}
