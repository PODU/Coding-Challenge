package archive

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"mytar/internal/ustar"
)

// TestFramingRoundTrip exercises the block framing layer directly: several
// members with varied body sizes, an empty file, a directory and a GNU long
// name, written then read back.
func TestFramingRoundTrip(t *testing.T) {
	type member struct {
		h    ustar.Header
		body string
	}
	longName := "deep/" + strings.Repeat("x", 120) + "/leaf.txt"
	members := []member{
		{ustar.Header{Name: "a.txt", Mode: 0o644, Size: 3, ModTime: time.Unix(1, 0), Typeflag: ustar.TypeReg}, "abc"},
		{ustar.Header{Name: "empty", Mode: 0o644, Size: 0, ModTime: time.Unix(2, 0), Typeflag: ustar.TypeReg}, ""},
		{ustar.Header{Name: "dir/", Mode: 0o755, Size: 0, ModTime: time.Unix(3, 0), Typeflag: ustar.TypeDir}, ""},
		{ustar.Header{Name: "exactly512", Mode: 0o644, Size: 512, ModTime: time.Unix(4, 0), Typeflag: ustar.TypeReg}, strings.Repeat("Z", 512)},
		{ustar.Header{Name: longName, Mode: 0o644, Size: 5, ModTime: time.Unix(5, 0), Typeflag: ustar.TypeReg}, "hello"},
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	for _, m := range members {
		h := m.h
		if !ustar.FitsUSTARName(h.Name) {
			if err := w.WriteGNULongName(h.Name); err != nil {
				t.Fatal(err)
			}
			// emulate the create layer truncating the real header name
			short := h.Name
			if len(short) > 100 {
				short = short[:100]
			}
			h.Name = short
		}
		if err := w.WriteHeader(&h); err != nil {
			t.Fatalf("WriteHeader %s: %v", m.h.Name, err)
		}
		if m.body != "" {
			if _, err := io.WriteString(w, m.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Archive must be a whole number of 512-byte blocks and end with two zero
	// blocks.
	if buf.Len()%ustar.BlockSize != 0 {
		t.Fatalf("archive length %d not a multiple of %d", buf.Len(), ustar.BlockSize)
	}
	tail := buf.Bytes()[buf.Len()-2*ustar.BlockSize:]
	for i, b := range tail {
		if b != 0 {
			t.Fatalf("trailing block byte %d != 0", i)
		}
	}

	r := NewReader(bytes.NewReader(buf.Bytes()))
	for i, m := range members {
		h, err := r.Next()
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		if h.Name != m.h.Name {
			t.Errorf("member %d name = %q want %q", i, h.Name, m.h.Name)
		}
		if h.Size != m.h.Size {
			t.Errorf("member %d size = %d want %d", i, h.Size, m.h.Size)
		}
		body, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read body %d: %v", i, err)
		}
		if string(body) != m.body {
			t.Errorf("member %d body = %q want %q", i, body, m.body)
		}
	}
	if _, err := r.Next(); err != io.EOF {
		t.Errorf("expected EOF after last member, got %v", err)
	}
}

func TestWriteHeaderEnforcesSize(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	h := ustar.Header{Name: "f", Mode: 0o644, Size: 2, ModTime: time.Unix(0, 0), Typeflag: ustar.TypeReg}
	if err := w.WriteHeader(&h); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("toolong")); err == nil {
		t.Error("expected error writing more than Size bytes")
	}
}
