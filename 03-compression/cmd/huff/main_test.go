package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCLIRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "input.txt")
	original := bytes.Repeat([]byte("compress me please, again and again. "), 500)
	writeFile(t, src, original)

	comp := filepath.Join(dir, "input.txt.huff")
	if code := run([]string{"compress", "-v", src}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("compress exit = %d", code)
	}
	ci, err := os.Stat(comp)
	if err != nil {
		t.Fatalf("expected %s: %v", comp, err)
	}
	if ci.Size() >= int64(len(original)) {
		t.Fatalf("compressed size %d not smaller than %d", ci.Size(), len(original))
	}

	out := filepath.Join(dir, "restored.txt")
	if code := run([]string{"decompress", "-o", out, comp}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("decompress exit = %d", code)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("decompressed output differs from original")
	}
}

func TestCLIDefaultDecompressOutput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "book.txt")
	writeFile(t, src, []byte("hello hello hello world"))
	if code := run([]string{"compress", src}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("compress exit = %d", code)
	}
	// Decompress book.txt.huff -> default strips .huff. Remove original first.
	os.Remove(src)
	if code := run([]string{"decompress", src + ".huff"}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("decompress exit = %d", code)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("expected default output %s: %v", src, err)
	}
}

func TestCLIUsageError(t *testing.T) {
	if code := run(nil, io.Discard, io.Discard); code != exitUsage {
		t.Fatalf("no args exit = %d, want %d", code, exitUsage)
	}
	if code := run([]string{"bogus"}, io.Discard, io.Discard); code != exitUsage {
		t.Fatalf("bad command exit = %d, want %d", code, exitUsage)
	}
	if code := run([]string{"compress"}, io.Discard, io.Discard); code != exitUsage {
		t.Fatalf("missing input exit = %d, want %d", code, exitUsage)
	}
}

func TestCLIMissingInput(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.txt")
	if code := run([]string{"compress", missing}, io.Discard, io.Discard); code != exitIO {
		t.Fatalf("missing file exit = %d, want %d", code, exitIO)
	}
}

func TestCLICorruptInput(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.huff")
	writeFile(t, bad, []byte("this is not a huffman file at all"))
	if code := run([]string{"decompress", "-o", filepath.Join(dir, "x"), bad}, io.Discard, io.Discard); code != exitCorrupt {
		t.Fatalf("corrupt input exit = %d, want %d", code, exitCorrupt)
	}
}

func TestCLIForceRefusal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src, []byte("data data data"))
	out := filepath.Join(dir, "a.txt.huff")
	writeFile(t, out, []byte("preexisting"))

	// Without -f: refuse with I/O exit code.
	if code := run([]string{"compress", src}, io.Discard, io.Discard); code != exitIO {
		t.Fatalf("overwrite without -f exit = %d, want %d", code, exitIO)
	}
	// With -f: succeed.
	if code := run([]string{"compress", "-f", src}, io.Discard, io.Discard); code != exitOK {
		t.Fatalf("overwrite with -f exit = %d, want %d", code, exitOK)
	}
}

func TestCLIHelp(t *testing.T) {
	var buf bytes.Buffer
	if code := run([]string{"--help"}, &buf, io.Discard); code != exitOK {
		t.Fatalf("help exit = %d", code)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Usage:")) {
		t.Fatal("help output missing usage")
	}
}

func TestCommas(t *testing.T) {
	cases := map[uint64]string{0: "0", 12: "12", 999: "999", 1000: "1,000", 3290880: "3,290,880"}
	for in, want := range cases {
		if got := commas(in); got != want {
			t.Errorf("commas(%d) = %q, want %q", in, got, want)
		}
	}
}
