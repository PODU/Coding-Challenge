package hash

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBytesKnownVectors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"abc": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
	}
	for in, want := range cases {
		if got := Bytes([]byte(in)); got != want {
			t.Errorf("Bytes(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestReaderMatchesBytes(t *testing.T) {
	t.Parallel()
	data := bytes.Repeat([]byte("dropbox-"), 100000) // 800 KB, streamed
	got, n, err := Reader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(data)) {
		t.Fatalf("size = %d, want %d", n, len(data))
	}
	if want := Bytes(data); got != want {
		t.Fatalf("streamed hash %s != one-shot %s", got, want)
	}
}

func TestFileEmptyAndContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := File(empty); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty file hash = %s", got)
	}

	abc := filepath.Join(dir, "abc")
	if err := os.WriteFile(abc, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := File(abc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(got, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad") {
		t.Fatalf("abc file hash = %s", got)
	}
}
