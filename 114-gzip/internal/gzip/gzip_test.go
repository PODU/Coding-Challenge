package gzip

import (
	"bytes"
	stdgzip "compress/gzip"
	"errors"
	"io"
	"math/rand"
	"testing"
)

func corpus() map[string][]byte {
	m := map[string][]byte{
		"empty":   {},
		"single":  {0x42},
		"text":    []byte("the quick brown fox jumps over the lazy dog\n"),
		"allsame": bytes.Repeat([]byte{'z'}, 10000),
		"repeat":  bytes.Repeat([]byte("Hello, gzip! "), 5000),
	}
	rng := rand.New(rand.NewSource(7))
	rnd := make([]byte, 40000)
	rng.Read(rnd)
	m["random"] = rnd

	big := make([]byte, 500000)
	for i := range big {
		big[i] = byte((i / 100) ^ (i * 3))
	}
	m["big"] = big
	return m
}

func roundTrip(t *testing.T, data []byte, level int) {
	t.Helper()
	var comp bytes.Buffer
	if err := Compress(&comp, data, level, Header{}); err != nil {
		t.Fatalf("Compress: %v", err)
	}
	var out bytes.Buffer
	if _, err := Decompress(&out, bytes.NewReader(comp.Bytes())); err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("round-trip mismatch: got %d, want %d bytes", out.Len(), len(data))
	}
}

func TestRoundTrip(t *testing.T) {
	for name, data := range corpus() {
		for _, level := range []int{0, 1, 6, 9} {
			t.Run(name, func(t *testing.T) { roundTrip(t, data, level) })
		}
	}
}

// Our gzip output must decode with the standard library.
func TestOurToStdlib(t *testing.T) {
	for name, data := range corpus() {
		for _, level := range []int{0, 1, 6, 9} {
			var comp bytes.Buffer
			if err := Compress(&comp, data, level, Header{Name: "file.bin"}); err != nil {
				t.Fatal(err)
			}
			zr, err := stdgzip.NewReader(bytes.NewReader(comp.Bytes()))
			if err != nil {
				t.Fatalf("%s L%d: stdlib NewReader: %v", name, level, err)
			}
			got, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("%s L%d: stdlib read: %v", name, level, err)
			}
			if err := zr.Close(); err != nil {
				t.Fatalf("%s L%d: stdlib close: %v", name, level, err)
			}
			if !bytes.Equal(got, data) {
				t.Fatalf("%s L%d: stdlib decode mismatch", name, level)
			}
			if zr.Name != "file.bin" {
				t.Errorf("%s: FNAME not preserved, got %q", name, zr.Name)
			}
		}
	}
}

// Standard-library gzip output must decode with our reader.
func TestStdlibToOurs(t *testing.T) {
	for name, data := range corpus() {
		for level := stdgzip.NoCompression; level <= stdgzip.BestCompression; level++ {
			var comp bytes.Buffer
			zw, _ := stdgzip.NewWriterLevel(&comp, level)
			zw.Name = "orig.dat"
			if _, err := zw.Write(data); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			hdr, err := Decompress(&out, bytes.NewReader(comp.Bytes()))
			if err != nil {
				t.Fatalf("%s L%d: our Decompress: %v", name, level, err)
			}
			if !bytes.Equal(out.Bytes(), data) {
				t.Fatalf("%s L%d: our decode mismatch", name, level)
			}
			if hdr.Name != "orig.dat" {
				t.Errorf("%s: FNAME not parsed, got %q", name, hdr.Name)
			}
		}
	}
}

func TestConcatenatedMembers(t *testing.T) {
	var comp bytes.Buffer
	a := []byte("first member\n")
	b := []byte("second member\n")
	if err := Compress(&comp, a, 6, Header{}); err != nil {
		t.Fatal(err)
	}
	if err := Compress(&comp, b, 9, Header{}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := Decompress(&out, bytes.NewReader(comp.Bytes())); err != nil {
		t.Fatalf("Decompress concatenated: %v", err)
	}
	want := append(append([]byte{}, a...), b...)
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("concatenated mismatch: %q", out.Bytes())
	}
}

func TestCorruptCRC(t *testing.T) {
	data := []byte("some data to checksum and then corrupt the trailer")
	var comp bytes.Buffer
	if err := Compress(&comp, data, 6, Header{}); err != nil {
		t.Fatal(err)
	}
	b := comp.Bytes()
	// Corrupt the CRC-32 field (4 bytes before the 4-byte ISIZE).
	b[len(b)-8] ^= 0xFF
	var out bytes.Buffer
	_, err := Decompress(&out, bytes.NewReader(b))
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("expected ErrChecksum, got %v", err)
	}
}

func TestCorruptISIZE(t *testing.T) {
	data := []byte("payload that has a known length recorded in ISIZE")
	var comp bytes.Buffer
	if err := Compress(&comp, data, 6, Header{}); err != nil {
		t.Fatal(err)
	}
	b := comp.Bytes()
	b[len(b)-1] ^= 0xFF // top byte of ISIZE
	var out bytes.Buffer
	_, err := Decompress(&out, bytes.NewReader(b))
	if !errors.Is(err, ErrSize) && !errors.Is(err, ErrChecksum) {
		t.Fatalf("expected ErrSize/ErrChecksum, got %v", err)
	}
}

func TestCorruptPayload(t *testing.T) {
	data := bytes.Repeat([]byte("compress me please "), 200)
	var comp bytes.Buffer
	if err := Compress(&comp, data, 6, Header{}); err != nil {
		t.Fatal(err)
	}
	b := comp.Bytes()
	b[len(b)/2] ^= 0xFF // flip a payload byte
	var out bytes.Buffer
	_, err := Decompress(&out, bytes.NewReader(b))
	if err == nil {
		t.Fatal("expected an error from corrupt payload, got nil")
	}
}

func TestTruncated(t *testing.T) {
	data := bytes.Repeat([]byte("truncate this stream"), 100)
	var comp bytes.Buffer
	if err := Compress(&comp, data, 6, Header{}); err != nil {
		t.Fatal(err)
	}
	b := comp.Bytes()[:comp.Len()-5]
	var out bytes.Buffer
	if _, err := Decompress(&out, bytes.NewReader(b)); err == nil {
		t.Fatal("expected error from truncated stream, got nil")
	}
}

func TestBadMagic(t *testing.T) {
	_, err := Decompress(io.Discard, bytes.NewReader([]byte{0x00, 0x01, 0x02, 0x03}))
	if !errors.Is(err, ErrHeader) {
		t.Fatalf("expected ErrHeader, got %v", err)
	}
}

func FuzzGzipRoundTrip(f *testing.F) {
	f.Add([]byte("hello"), 6)
	f.Add([]byte{}, 0)
	f.Fuzz(func(t *testing.T, data []byte, level int) {
		level = ((level % 10) + 10) % 10
		var comp bytes.Buffer
		if err := Compress(&comp, data, level, Header{}); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if _, err := Decompress(&out, bytes.NewReader(comp.Bytes())); err != nil {
			t.Fatalf("decompress: %v", err)
		}
		if !bytes.Equal(out.Bytes(), data) {
			t.Fatal("round-trip mismatch")
		}
		// Cross-check against the standard library.
		zr, err := stdgzip.NewReader(bytes.NewReader(comp.Bytes()))
		if err != nil {
			t.Fatalf("stdlib reader: %v", err)
		}
		got, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("stdlib read: %v", err)
		}
		if !bytes.Equal(got, data) {
			t.Fatal("stdlib decode mismatch")
		}
	})
}
