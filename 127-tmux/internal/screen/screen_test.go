package screen

import (
	"fmt"
	"testing"
)

func TestWritePlainText(t *testing.T) {
	s := New(10, 3)
	s.Write([]byte("hello"))
	if got := s.Line(0); got != "hello" {
		t.Fatalf("line0 = %q", got)
	}
	cx, cy := s.Cursor()
	if cx != 5 || cy != 0 {
		t.Fatalf("cursor = %d,%d", cx, cy)
	}
}

func TestNewlineAndCarriageReturn(t *testing.T) {
	s := New(10, 3)
	s.Write([]byte("ab\r\ncd"))
	if s.Line(0) != "ab" || s.Line(1) != "cd" {
		t.Fatalf("lines = %q / %q", s.Line(0), s.Line(1))
	}
}

func TestWrapAtWidth(t *testing.T) {
	s := New(3, 3)
	s.Write([]byte("abcd"))
	if s.Line(0) != "abc" || s.Line(1) != "d" {
		t.Fatalf("wrap wrong: %q / %q", s.Line(0), s.Line(1))
	}
}

func TestScrollbackCaptured(t *testing.T) {
	s := New(5, 2)
	s.Write([]byte("one\r\ntwo\r\nthree"))
	// "one" scrolled off the top.
	if s.ScrollbackLen() != 1 {
		t.Fatalf("scrollback len = %d", s.ScrollbackLen())
	}
	if s.Scrollback()[0] != "one" {
		t.Fatalf("scrollback[0] = %q", s.Scrollback()[0])
	}
	if s.Line(0) != "two" || s.Line(1) != "three" {
		t.Fatalf("visible = %q / %q", s.Line(0), s.Line(1))
	}
}

func TestCursorPositionEscape(t *testing.T) {
	s := New(10, 5)
	s.Write([]byte("\x1b[3;5H")) // row 3 col 5 (1-based)
	cx, cy := s.Cursor()
	if cx != 4 || cy != 2 {
		t.Fatalf("cursor = %d,%d", cx, cy)
	}
	s.Write([]byte("X"))
	if s.Cell(4, 2).Rune != 'X' {
		t.Fatalf("X not placed at 4,2")
	}
}

func TestEraseDisplay(t *testing.T) {
	s := New(5, 3)
	s.Write([]byte("aaaaa\r\nbbbbb\r\nccccc"))
	s.Write([]byte("\x1b[H"))  // home
	s.Write([]byte("\x1b[2J")) // clear all
	for y := 0; y < 3; y++ {
		if s.Line(y) != "" {
			t.Fatalf("row %d not cleared: %q", y, s.Line(y))
		}
	}
}

func TestEraseLineToEnd(t *testing.T) {
	s := New(6, 1)
	s.Write([]byte("abcdef"))
	s.Write([]byte("\x1b[1;4H")) // col 4 (index 3)
	s.Write([]byte("\x1b[0K"))   // erase to end
	if s.Line(0) != "abc" {
		t.Fatalf("line = %q", s.Line(0))
	}
}

func TestSGRStyleCaptured(t *testing.T) {
	s := New(5, 1)
	s.Write([]byte("\x1b[1;31mR\x1b[0mx"))
	if s.Cell(0, 0).Style != "1;31" {
		t.Fatalf("style = %q", s.Cell(0, 0).Style)
	}
	if s.Cell(1, 0).Style != "" {
		t.Fatalf("reset style = %q", s.Cell(1, 0).Style)
	}
}

func TestResizePreservesContent(t *testing.T) {
	s := New(5, 2)
	s.Write([]byte("hi\r\nyo"))
	s.Resize(8, 3)
	if s.Line(0) != "hi" || s.Line(1) != "yo" {
		t.Fatalf("after resize: %q / %q", s.Line(0), s.Line(1))
	}
	w, h := s.Size()
	if w != 8 || h != 3 {
		t.Fatalf("size = %dx%d", w, h)
	}
}

func TestCursorMovementRelative(t *testing.T) {
	s := New(10, 5)
	s.Write([]byte("\x1b[3B\x1b[4C")) // down 3, right 4
	cx, cy := s.Cursor()
	if cx != 4 || cy != 3 {
		t.Fatalf("cursor = %d,%d", cx, cy)
	}
}

// Sanity: a moderately complex sequence lands where expected.
func TestExampleFormatting(t *testing.T) {
	s := New(20, 4)
	fmt.Fprint(writerFor(s), "$ ls\r\nfile1  file2\r\n$ ")
	if s.Line(0) != "$ ls" || s.Line(1) != "file1  file2" || s.Line(2) != "$" {
		t.Fatalf("prompt render: %q / %q / %q", s.Line(0), s.Line(1), s.Line(2))
	}
}

type screenWriter struct{ s *Screen }

func (w screenWriter) Write(p []byte) (int, error) { w.s.Write(p); return len(p), nil }
func writerFor(s *Screen) screenWriter             { return screenWriter{s} }
