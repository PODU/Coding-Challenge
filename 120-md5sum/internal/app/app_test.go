package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run is a helper that invokes Run with a string stdin and captures streams.
func run(args []string, stdin string) (out, errOut string, code int) {
	var o, e bytes.Buffer
	code = Run(args, strings.NewReader(stdin), &o, &e)
	return o.String(), e.String(), code
}

func TestDefaultFormatStdin(t *testing.T) {
	out, _, code := run(nil, "abc")
	want := "900150983cd24fb0d6963f7d28e17f72  -\n"
	if out != want || code != 0 {
		t.Fatalf("got %q code %d, want %q", out, code, want)
	}
}

func TestEmptyStdin(t *testing.T) {
	out, _, code := run(nil, "")
	want := "d41d8cd98f00b204e9800998ecf8427e  -\n"
	if out != want || code != 0 {
		t.Fatalf("got %q code %d", out, code)
	}
}

func TestExplicitDash(t *testing.T) {
	out, _, _ := run([]string{"-"}, "abc")
	if !strings.HasSuffix(out, "  -\n") {
		t.Fatalf("expected name '-', got %q", out)
	}
}

func TestFormatVariants(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file with spaces.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const hexHello = "5d41402abc4b2a76b9719d911017c592"

	// Default
	out, _, _ := run([]string{f}, "")
	if out != hexHello+"  "+f+"\n" {
		t.Errorf("default: %q", out)
	}
	// Binary
	out, _, _ = run([]string{"-b", f}, "")
	if out != hexHello+" *"+f+"\n" {
		t.Errorf("binary: %q", out)
	}
	// Tag
	out, _, _ = run([]string{"--tag", f}, "")
	if out != "MD5 ("+f+") = "+hexHello+"\n" {
		t.Errorf("tag: %q", out)
	}
}

func TestMissingFileNormalMode(t *testing.T) {
	out, errOut, code := run([]string{"/no/such/file/here"}, "")
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if out != "" {
		t.Errorf("expected no stdout, got %q", out)
	}
	if !strings.Contains(errOut, "md5sum:") {
		t.Errorf("expected stderr diagnostic, got %q", errOut)
	}
}

func TestMixedFilesContinueOnError(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	os.WriteFile(good, []byte("hello"), 0o644)

	out, _, code := run([]string{"/nope", good}, "")
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if !strings.Contains(out, "5d41402abc4b2a76b9719d911017c592") {
		t.Errorf("good file should still be hashed: %q", out)
	}
}

// writeSums hashes files and returns a checksum-file body in the given mode.
func TestCheckPass(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0o644)

	sums := "5d41402abc4b2a76b9719d911017c592  " + a + "\n"
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, []byte(sums), 0o644)

	out, _, code := run([]string{"-c", cf}, "")
	if code != 0 {
		t.Fatalf("want exit 0, got %d (out=%q)", code, out)
	}
	if out != a+": OK\n" {
		t.Errorf("got %q", out)
	}
}

func TestCheckFail(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0o644)

	// Wrong digest.
	sums := "00000000000000000000000000000000  " + a + "\n"
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, []byte(sums), 0o644)

	out, errOut, code := run([]string{"-c", cf}, "")
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if out != a+": FAILED\n" {
		t.Errorf("stdout %q", out)
	}
	if !strings.Contains(errOut, "did NOT match") {
		t.Errorf("expected mismatch warning, got %q", errOut)
	}
}

func TestCheckMissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "ghost.txt")
	sums := "00000000000000000000000000000000  " + missing + "\n"
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, []byte(sums), 0o644)

	out, errOut, code := run([]string{"-c", cf}, "")
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if !strings.Contains(out, "FAILED open or read") {
		t.Errorf("stdout %q", out)
	}
	if !strings.Contains(errOut, "could not be read") {
		t.Errorf("stderr %q", errOut)
	}
}

func TestCheckMalformedLine(t *testing.T) {
	dir := t.TempDir()
	cf := filepath.Join(dir, "bad.md5")
	os.WriteFile(cf, []byte("this is not a checksum line\n"), 0o644)

	_, errOut, code := run([]string{"-c", cf}, "")
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "improperly formatted") {
		t.Errorf("stderr %q", errOut)
	}
}

func TestCheckQuiet(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0o644)
	sums := "5d41402abc4b2a76b9719d911017c592  " + a + "\n"
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, []byte(sums), 0o644)

	out, _, code := run([]string{"-c", "--quiet", cf}, "")
	if code != 0 || out != "" {
		t.Fatalf("quiet OK should be silent: out=%q code=%d", out, code)
	}
}

func TestCheckStatus(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0o644)
	// mismatch
	sums := "00000000000000000000000000000000  " + a + "\n"
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, []byte(sums), 0o644)

	out, errOut, code := run([]string{"-c", "--status", cf}, "")
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if out != "" || errOut != "" {
		t.Errorf("status should be silent: out=%q err=%q", out, errOut)
	}
}

// TestTagRoundTrip: --tag output is re-parsable by -c.
func TestTagRoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0o644)

	tagOut, _, _ := run([]string{"--tag", a}, "")
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, []byte(tagOut), 0o644)

	out, _, code := run([]string{"-c", cf}, "")
	if code != 0 || out != a+": OK\n" {
		t.Fatalf("tag round-trip failed: out=%q code=%d", out, code)
	}
}

// TestBinaryRoundTrip: -b output is re-parsable by -c.
func TestBinaryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0o644)

	binOut, _, _ := run([]string{"-b", a}, "")
	if !strings.Contains(binOut, " *") {
		t.Fatalf("expected binary marker, got %q", binOut)
	}
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, []byte(binOut), 0o644)

	out, _, code := run([]string{"-c", cf}, "")
	if code != 0 || out != a+": OK\n" {
		t.Fatalf("binary round-trip failed: out=%q code=%d", out, code)
	}
}

// TestCRLFTolerance: checksum files with CRLF line endings still verify.
func TestCRLFTolerance(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0o644)
	sums := "5d41402abc4b2a76b9719d911017c592  " + a + "\r\n"
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, []byte(sums), 0o644)

	out, _, code := run([]string{"-c", cf}, "")
	if code != 0 || out != a+": OK\n" {
		t.Fatalf("CRLF tolerance failed: out=%q code=%d", out, code)
	}
}

func TestMutuallyExclusiveFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--tag", "-c"},
		{"--tag", "-b"},
	} {
		_, errOut, code := run(args, "")
		if code == 0 {
			t.Errorf("args %v: expected usage error", args)
		}
		if !strings.Contains(errOut, "md5sum:") {
			t.Errorf("args %v: expected diagnostic, got %q", args, errOut)
		}
	}
}

func TestUnknownAlgorithm(t *testing.T) {
	_, _, code := run([]string{"-a", "crc32"}, "")
	if code == 0 {
		t.Fatal("expected error for unknown algorithm")
	}
}

func TestSHA256Algorithm(t *testing.T) {
	out, _, code := run([]string{"-a", "sha256"}, "abc")
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad  -\n"
	if out != want || code != 0 {
		t.Fatalf("got %q code %d, want %q", out, code, want)
	}
}

func TestSHA256TagLabel(t *testing.T) {
	out, _, _ := run([]string{"-a", "sha256", "--tag"}, "abc")
	if !strings.HasPrefix(out, "SHA256 (-) = ") {
		t.Fatalf("expected SHA256 tag label, got %q", out)
	}
}

func TestHelpAndVersion(t *testing.T) {
	out, _, code := run([]string{"--help"}, "")
	if code != 0 || !strings.Contains(out, "Usage:") {
		t.Errorf("help: out=%q code=%d", out, code)
	}
	out, _, code = run([]string{"--version"}, "")
	if code != 0 || !strings.Contains(out, "md5sum") {
		t.Errorf("version: out=%q code=%d", out, code)
	}
}

func TestParseClusteredFlags(t *testing.T) {
	opts, err := parseArgs([]string{"-bc", "file"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.binary || !opts.check {
		t.Errorf("clustered -bc not parsed: %+v", opts)
	}
}

func TestParseCheckLineFormats(t *testing.T) {
	algo := md5Algo()
	cases := []struct {
		line   string
		ok     bool
		name   string
		binary bool
	}{
		{"5d41402abc4b2a76b9719d911017c592  a.txt", true, "a.txt", false},
		{"5d41402abc4b2a76b9719d911017c592 *a.txt", true, "a.txt", true},
		{"MD5 (a.txt) = 5d41402abc4b2a76b9719d911017c592", true, "a.txt", true},
		{"5d41402abc4b2a76b9719d911017c592  name with spaces.txt", true, "name with spaces.txt", false},
		{"garbage", false, "", false},
		{"", false, "", false},
		{"5d41402abc4b2a76b9719d911017c59  short.txt", false, "", false}, // 31 hex chars
	}
	for _, c := range cases {
		entry, ok := parseCheckLine(c.line, algo)
		if ok != c.ok {
			t.Errorf("line %q: ok=%v want %v", c.line, ok, c.ok)
			continue
		}
		if ok && (entry.name != c.name || entry.binary != c.binary) {
			t.Errorf("line %q: name=%q binary=%v want name=%q binary=%v",
				c.line, entry.name, entry.binary, c.name, c.binary)
		}
	}
}
