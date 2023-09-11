package index

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"path/filepath"
	"testing"
)

func sampleEntry(path, oidHex string) Entry {
	e := Entry{
		CtimeSec:  1700000000,
		CtimeNsec: 123,
		MtimeSec:  1700000001,
		MtimeNsec: 456,
		Mode:      ModeRegular,
		Size:      6,
		Path:      path,
	}
	_ = e.SetOIDHex(oidHex)
	return e
}

func TestRoundTrip(t *testing.T) {
	idx := New()
	idx.Add(sampleEntry("hello.txt", "ce013625030ba8dba906f756967f9e9ca394464a"))
	idx.Add(sampleEntry("src/main.go", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"))
	idx.Add(sampleEntry("a.txt", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"))

	data := idx.Bytes()
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(parsed.Entries))
	}
	// Sorted by path.
	wantOrder := []string{"a.txt", "hello.txt", "src/main.go"}
	for i, w := range wantOrder {
		if parsed.Entries[i].Path != w {
			t.Fatalf("entry %d = %s, want %s", i, parsed.Entries[i].Path, w)
		}
	}
	// Field fidelity.
	for i := range idx.Entries {
		a, b := idx.Entries[i], parsed.Entries[i]
		if a != b {
			t.Fatalf("entry %d mismatch:\n got %+v\nwant %+v", i, b, a)
		}
	}
}

func TestHeaderAndChecksum(t *testing.T) {
	idx := New()
	idx.Add(sampleEntry("file", "ce013625030ba8dba906f756967f9e9ca394464a"))
	data := idx.Bytes()

	if string(data[:4]) != "DIRC" {
		t.Fatalf("signature = %q", data[:4])
	}
	if v := binary.BigEndian.Uint32(data[4:8]); v != 2 {
		t.Fatalf("version = %d", v)
	}
	if c := binary.BigEndian.Uint32(data[8:12]); c != 1 {
		t.Fatalf("count = %d", c)
	}
	body := data[:len(data)-20]
	sum := sha1.Sum(body)
	if !bytes.Equal(sum[:], data[len(data)-20:]) {
		t.Fatalf("trailing checksum invalid")
	}
}

func TestEntryPaddingAlignment(t *testing.T) {
	// Every entry block (62 + len(path) + padding) must be a multiple of 8 with
	// at least one NUL terminator.
	for _, path := range []string{"a", "ab", "abc", "abcd", "abcde", "abcdef", "1234567", "12345678", "123456789"} {
		idx := New()
		idx.Add(sampleEntry(path, "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"))
		data := idx.Bytes()
		// Total = 12 header + entry + 20 trailer.
		entryLen := len(data) - 12 - 20
		if entryLen%8 != 0 {
			t.Fatalf("path %q: entry length %d not multiple of 8", path, entryLen)
		}
		minLen := 62 + len(path)
		if entryLen <= minLen-1 {
			t.Fatalf("path %q: entry length %d has no NUL terminator (min %d)", path, entryLen, minLen)
		}
	}
}

func TestReadMissingFileEmptyIndex(t *testing.T) {
	idx, err := Read(filepath.Join(t.TempDir(), "nonexistent-index"))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Entries) != 0 {
		t.Fatalf("expected empty index, got %d entries", len(idx.Entries))
	}
}

func TestCorruptChecksumRejected(t *testing.T) {
	idx := New()
	idx.Add(sampleEntry("file", "ce013625030ba8dba906f756967f9e9ca394464a"))
	data := idx.Bytes()
	data[len(data)-1] ^= 0xFF // corrupt checksum
	if _, err := Parse(data); err == nil {
		t.Fatal("expected checksum error, got nil")
	}
}

func TestWriteReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index")
	idx := New()
	idx.Add(sampleEntry("x", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"))
	if err := idx.Write(path); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Path != "x" {
		t.Fatalf("round-trip via file failed: %+v", got.Entries)
	}
}
