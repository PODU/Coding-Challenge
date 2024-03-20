package bloom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func buildSample(t *testing.T) (*BloomFilter, []string) {
	t.Helper()
	bf, err := New(1000, 0.01)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "café"}
	for _, w := range words {
		bf.Add(w)
	}
	return bf, words
}

// TestRoundTrip builds a filter, writes it, reads it back, and asserts identical
// m, k, and bitset bytes, plus identical Contains answers for present and absent
// words.
func TestRoundTrip(t *testing.T) {
	bf, words := buildSample(t)

	var buf bytes.Buffer
	n, err := bf.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if int(n) != buf.Len() {
		t.Errorf("WriteTo returned %d bytes, buffer has %d", n, buf.Len())
	}

	var loaded BloomFilter
	rn, err := loaded.ReadFrom(&buf)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if rn != n {
		t.Errorf("ReadFrom read %d bytes, WriteTo wrote %d", rn, n)
	}

	if loaded.M() != bf.M() {
		t.Errorf("m mismatch: got %d, want %d", loaded.M(), bf.M())
	}
	if loaded.K() != bf.K() {
		t.Errorf("k mismatch: got %d, want %d", loaded.K(), bf.K())
	}
	if !bytes.Equal(loaded.Bits(), bf.Bits()) {
		t.Errorf("bitset mismatch after round-trip")
	}

	for _, w := range words {
		if !loaded.Contains(w) {
			t.Errorf("loaded filter false negative for %q", w)
		}
	}
	for _, w := range []string{"zzzznotpresent", "qqqqabsent", "nope12345"} {
		if loaded.Contains(w) != bf.Contains(w) {
			t.Errorf("answer differs for absent word %q: loaded=%v orig=%v",
				w, loaded.Contains(w), bf.Contains(w))
		}
	}
}

// TestFormatConformance asserts the on-disk bytes match the documented layout.
func TestFormatConformance(t *testing.T) {
	bf, _ := buildSample(t)
	var buf bytes.Buffer
	if _, err := bf.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	if got := string(data[0:4]); got != "CCBF" {
		t.Errorf("magic = %q, want CCBF", got)
	}
	if v := binary.BigEndian.Uint16(data[4:6]); v != formatVersion {
		t.Errorf("version = %d, want %d", v, formatVersion)
	}
	if k := binary.BigEndian.Uint16(data[6:8]); k != bf.K() {
		t.Errorf("k = %d, want %d", k, bf.K())
	}
	if m := binary.BigEndian.Uint32(data[8:12]); m != bf.M() {
		t.Errorf("m = %d, want %d", m, bf.M())
	}
	wantLen := headerSize + (int(bf.M())+7)/8
	if len(data) != wantLen {
		t.Errorf("file length = %d, want %d", len(data), wantLen)
	}
}

func TestBadMagic(t *testing.T) {
	bf, _ := buildSample(t)
	var buf bytes.Buffer
	bf.WriteTo(&buf)
	data := buf.Bytes()
	data[0] = 'X' // corrupt magic

	var loaded BloomFilter
	_, err := loaded.ReadFrom(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for bad magic, got nil")
	}
	var fe *FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *FormatError, got %T: %v", err, err)
	}
}

func TestUnsupportedVersion(t *testing.T) {
	bf, _ := buildSample(t)
	var buf bytes.Buffer
	bf.WriteTo(&buf)
	data := buf.Bytes()
	binary.BigEndian.PutUint16(data[4:6], 999) // bogus version

	var loaded BloomFilter
	_, err := loaded.ReadFrom(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for unsupported version, got nil")
	}
	var fe *FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *FormatError, got %T", err)
	}
}

func TestTruncatedBitArray(t *testing.T) {
	bf, _ := buildSample(t)
	var buf bytes.Buffer
	bf.WriteTo(&buf)
	data := buf.Bytes()
	// Keep a valid header but lop off most of the bit array.
	truncated := data[:headerSize+4]

	var loaded BloomFilter
	_, err := loaded.ReadFrom(bytes.NewReader(truncated))
	if err == nil {
		t.Fatal("expected error for truncated bit array, got nil")
	}
	var fe *FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *FormatError, got %T", err)
	}
}

func TestTruncatedHeader(t *testing.T) {
	var loaded BloomFilter
	_, err := loaded.ReadFrom(bytes.NewReader([]byte("CC")))
	if err == nil {
		t.Fatal("expected error for truncated header, got nil")
	}
	var fe *FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *FormatError, got %T", err)
	}
}

func TestEmptyReader(t *testing.T) {
	var loaded BloomFilter
	_, err := loaded.ReadFrom(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("expected error for empty reader")
	}
	if errors.Is(err, io.EOF) {
		// acceptable underlying cause, but we wrap it
	}
}
