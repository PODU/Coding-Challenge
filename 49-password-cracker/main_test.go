package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"passcrack/hashreg"
)

func md5hex(s string) string {
	a, _ := hashreg.Get("md5")
	return hashreg.HexSum(a, []byte(s))
}

// runArgs invokes the CLI entry point and returns exit code, stdout, stderr.
func runArgs(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// Exit code 0 + pipe-clean stdout when the hash is cracked.
func TestCLICrackedExitZero(t *testing.T) {
	target := md5hex("hunter2")
	code, stdout, _ := runArgs("-q", "-m", "dict", "-w", "testdata/words.txt", target)
	if code != exitAllCracked {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := target + ":hunter2\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

// stdout must contain ONLY hash:plaintext lines (banner/progress go to stderr).
func TestCLIStdoutPipeClean(t *testing.T) {
	target := md5hex("apple")
	_, stdout, stderr := runArgs("-m", "dict", "-w", "testdata/words.txt", target)
	if strings.TrimSpace(stdout) != target+":apple" {
		t.Fatalf("stdout not pipe-clean: %q", stdout)
	}
	if !strings.Contains(stderr, "AUTHORIZED USE ONLY") {
		t.Fatalf("ethics banner not on stderr: %q", stderr)
	}
}

// Exit code 1 when a target is left uncracked.
func TestCLIUncrackedExitOne(t *testing.T) {
	target := md5hex("not-in-list")
	code, stdout, _ := runArgs("-q", "-m", "dict", "-w", "testdata/words.txt", target)
	if code != exitUncracked {
		t.Fatalf("exit = %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

// Exit code 2 for usage/validation errors.
func TestCLIUsageErrors(t *testing.T) {
	cases := [][]string{
		{"-q", "-a", "bogus", "-w", "testdata/words.txt", md5hex("apple")}, // bad algo
		{"-q", "-m", "dict", "-w", "testdata/words.txt", "nothex"},         // malformed hash
		{"-q", "-m", "dict", "-w", "testdata/words.txt"},                   // no target
		{"-q", "-m", "dict", md5hex("apple")},                              // dict without wordlist
		{"-q", "-m", "weird", "-w", "testdata/words.txt", md5hex("apple")}, // bad mode
	}
	for i, args := range cases {
		code, _, _ := runArgs(args...)
		if code != exitUsage {
			t.Errorf("case %d: exit = %d, want 2 (args=%v)", i, code, args)
		}
	}
}

// Brute mode recovers a challenge vector and exits 0.
func TestCLIBruteChallengeVector(t *testing.T) {
	code, stdout, _ := runArgs("-q", "-m", "brute",
		"--charset", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "--max", "4",
		"7a95bf926a0333f57705aeac07a362a2")
	if code != exitAllCracked {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(stdout, ":PASS") {
		t.Fatalf("stdout = %q, want PASS", stdout)
	}
}

// --hashes-file with multiple targets, plus auto algorithm detection.
func TestCLIHashesFileAndAuto(t *testing.T) {
	dir := t.TempDir()
	hf := filepath.Join(dir, "hashes.txt")
	body := md5hex("apple") + "\n# comment\n" + md5hex("banana") + "\n"
	if err := os.WriteFile(hf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runArgs("-q", "-a", "auto", "-m", "dict",
		"-w", "testdata/words.txt", "--hashes-file", hf)
	if code != exitAllCracked {
		t.Fatalf("exit = %d, want 0; stdout=%q", code, stdout)
	}
	if !strings.Contains(stdout, ":apple") || !strings.Contains(stdout, ":banana") {
		t.Fatalf("stdout missing results: %q", stdout)
	}
}

// Rainbow build then lookup via the CLI.
func TestCLIRainbowBuildAndLookup(t *testing.T) {
	dir := t.TempDir()
	rt := filepath.Join(dir, "rt.tsv")
	if code, _, se := runArgs("-q", "-m", "rainbow", "--build-rainbow",
		"-w", "testdata/words.txt", "--rainbow-file", rt); code != 0 {
		t.Fatalf("build exit = %d, stderr=%q", code, se)
	}
	target := md5hex("hunter2")
	code, stdout, _ := runArgs("-q", "-m", "rainbow", "--rainbow-file", rt, target)
	if code != exitAllCracked {
		t.Fatalf("lookup exit = %d, want 0", code)
	}
	if !strings.Contains(stdout, target+":hunter2") {
		t.Fatalf("rainbow lookup stdout = %q", stdout)
	}
}
