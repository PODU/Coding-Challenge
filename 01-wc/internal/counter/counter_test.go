package counter

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Counts
	}{
		// Empty input -> all zeros.
		{"empty", "", Counts{Bytes: 0, Lines: 0, Words: 0, Chars: 0}},
		// No trailing newline -> last line not counted as a line.
		{"no trailing newline", "hello world", Counts{Bytes: 11, Lines: 0, Words: 2, Chars: 11}},
		{"three lines", "a\nb\nc\n", Counts{Bytes: 6, Lines: 3, Words: 3, Chars: 6}},
		// Whitespace only -> zero words.
		{"whitespace only", "   \n\t \n", Counts{Bytes: 7, Lines: 2, Words: 0, Chars: 7}},
		// Multibyte UTF-8 -> chars < bytes.
		{"utf8 accented", "héllo\n", Counts{Bytes: 7, Lines: 1, Words: 1, Chars: 6}},
		{"utf8 cjk", "日本語\n", Counts{Bytes: 10, Lines: 1, Words: 1, Chars: 4}},
		// Multiple consecutive whitespace -> one word per run.
		{"runs of spaces", "  foo   bar\t\tbaz  ", Counts{Bytes: 18, Lines: 0, Words: 3, Chars: 18}},
		// CRLF line endings.
		{"crlf", "a b\r\nc\r\n", Counts{Bytes: 8, Lines: 2, Words: 3, Chars: 8}},
		// Leading/trailing newlines.
		{"only newlines", "\n\n\n", Counts{Bytes: 3, Lines: 3, Words: 0, Chars: 3}},
		// NUL bytes are ordinary non-whitespace bytes.
		{"nul bytes", "a\x00b\n", Counts{Bytes: 4, Lines: 1, Words: 1, Chars: 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Count(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Count returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Count(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

// A truncated/invalid UTF-8 sequence must not panic and each undecodable
// byte counts as one character.
func TestCountInvalidUTF8(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantChars int64
		wantBytes int64
	}{
		// Lone continuation byte: 1 invalid byte -> 1 char.
		{"lone continuation", []byte{0x80}, 1, 1},
		// 'a' + truncated 2-byte lead (0xC3 with no continuation) at EOF.
		{"truncated lead at eof", []byte{'a', 0xC3}, 2, 2},
		// Valid rune followed by invalid byte then valid.
		{"mixed", append([]byte("é"), 0xFF, 'x'), 3, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Count(bytes.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Count returned error: %v", err)
			}
			if got.Chars != tt.wantChars {
				t.Errorf("Chars = %d, want %d", got.Chars, tt.wantChars)
			}
			if got.Bytes != tt.wantBytes {
				t.Errorf("Bytes = %d, want %d", got.Bytes, tt.wantBytes)
			}
		})
	}
}

// chunkReader yields its data one byte at a time to exercise the cross-buffer
// carry logic for runes and the inWord state machine spanning Read calls.
type chunkReader struct {
	data []byte
	pos  int
	size int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := r.size
	if n > len(p) {
		n = len(p)
	}
	if r.pos+n > len(r.data) {
		n = len(r.data) - r.pos
	}
	copy(p, r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

// A multibyte rune split across read boundaries must still count as one char.
func TestCountRuneSplitAcrossReads(t *testing.T) {
	// "a日本b" with a multibyte CJK rune; feed one byte per Read.
	data := []byte("a日本b\n")
	got, err := Count(&chunkReader{data: data, size: 1})
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	want := Counts{Bytes: int64(len(data)), Lines: 1, Words: 1, Chars: 5}
	if got != want {
		t.Errorf("split-read Count = %+v, want %+v", got, want)
	}
}

// Words spanning a buffer boundary must not be double-counted.
func TestCountWordAcrossReads(t *testing.T) {
	data := []byte("foobar baz")
	got, err := Count(&chunkReader{data: data, size: 3})
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if got.Words != 2 {
		t.Errorf("Words = %d, want 2", got.Words)
	}
}

func TestAdd(t *testing.T) {
	a := Counts{Bytes: 1, Lines: 2, Words: 3, Chars: 4}
	a.Add(Counts{Bytes: 10, Lines: 20, Words: 30, Chars: 40})
	want := Counts{Bytes: 11, Lines: 22, Words: 33, Chars: 44}
	if a != want {
		t.Errorf("Add = %+v, want %+v", a, want)
	}
}

// errReader returns an error partway through to verify error propagation.
type errReader struct{ err error }

func (e errReader) Read(p []byte) (int, error) { return 0, e.err }

func TestCountReadError(t *testing.T) {
	sentinel := io.ErrUnexpectedEOF
	_, err := Count(errReader{err: sentinel})
	if err != sentinel {
		t.Errorf("Count error = %v, want %v", err, sentinel)
	}
}
