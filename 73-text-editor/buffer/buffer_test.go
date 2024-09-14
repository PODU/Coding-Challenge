package buffer

import (
	"bytes"
	"strings"
	"testing"
)

func bufFrom(lines ...string) *Buffer {
	b := New()
	for _, l := range lines {
		b.appendRow([]byte(l))
	}
	return b
}

func TestInsertCharEmptyBuffer(t *testing.T) {
	b := New()
	b.InsertChar(0, 0, 'X') // cy == NumRows -> creates row 0
	if b.NumRows() != 1 {
		t.Fatalf("NumRows = %d, want 1", b.NumRows())
	}
	if got := b.Row(0).String(); got != "X" {
		t.Fatalf("row 0 = %q, want %q", got, "X")
	}
}

func TestInsertCharMidAndEnd(t *testing.T) {
	b := bufFrom("helo")
	b.InsertChar(0, 2, 'l') // -> "hello"
	if got := b.Row(0).String(); got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
	b.InsertChar(0, 5, '!') // end of row
	if got := b.Row(0).String(); got != "hello!" {
		t.Fatalf("got %q, want %q", got, "hello!")
	}
	if got := b.Row(0).Render(); got != "hello!" {
		t.Fatalf("render = %q", got)
	}
}

func TestInsertCharUTF8(t *testing.T) {
	b := bufFrom("ab")
	b.InsertChar(0, 1, 'é')
	if got := b.Row(0).String(); got != "aéb" {
		t.Fatalf("got %q, want %q", got, "aéb")
	}
}

func TestDeleteCharMid(t *testing.T) {
	b := bufFrom("hello")
	b.DeleteChar(0, 3) // delete char left of col 3 -> remove 'l' at index 2
	if got := b.Row(0).String(); got != "helo" {
		t.Fatalf("got %q, want %q", got, "helo")
	}
}

func TestDeleteCharAtZeroNoop(t *testing.T) {
	b := bufFrom("abc")
	b.DeleteChar(0, 0)
	if got := b.Row(0).String(); got != "abc" {
		t.Fatalf("got %q, want unchanged abc", got)
	}
	if b.NumRows() != 1 {
		t.Fatalf("NumRows = %d, want 1", b.NumRows())
	}
}

func TestDeleteCharJoinsRows(t *testing.T) {
	b := bufFrom("foo", "bar")
	b.DeleteChar(1, 0) // join row 1 onto row 0
	if b.NumRows() != 1 {
		t.Fatalf("NumRows = %d, want 1", b.NumRows())
	}
	if got := b.Row(0).String(); got != "foobar" {
		t.Fatalf("got %q, want %q", got, "foobar")
	}
}

func TestInsertNewlineSplit(t *testing.T) {
	b := bufFrom("foobar")
	b.InsertNewline(0, 3)
	if b.NumRows() != 2 {
		t.Fatalf("NumRows = %d, want 2", b.NumRows())
	}
	if b.Row(0).String() != "foo" || b.Row(1).String() != "bar" {
		t.Fatalf("rows = %q, %q", b.Row(0).String(), b.Row(1).String())
	}
}

func TestInsertNewlineAtZeroOpensAbove(t *testing.T) {
	b := bufFrom("abc")
	b.InsertNewline(0, 0)
	if b.NumRows() != 2 {
		t.Fatalf("NumRows = %d, want 2", b.NumRows())
	}
	if b.Row(0).String() != "" || b.Row(1).String() != "abc" {
		t.Fatalf("rows = %q, %q", b.Row(0).String(), b.Row(1).String())
	}
}

func TestInsertNewlineAtEndOpensBelow(t *testing.T) {
	b := bufFrom("abc")
	b.InsertNewline(0, 3)
	if b.NumRows() != 2 || b.Row(0).String() != "abc" || b.Row(1).String() != "" {
		t.Fatalf("rows = %q, %q", b.Row(0).String(), b.Row(1).String())
	}
}

func TestRenderTabExpansion(t *testing.T) {
	b := bufFrom("a\tb")
	// 'a' at col 0; tab fills to col 8; 'b' at col 8.
	want := "a       b"
	if got := b.Row(0).Render(); got != want {
		t.Fatalf("render = %q, want %q", got, want)
	}
}

func TestCxRxRoundTrip(t *testing.T) {
	r := &Row{chars: []byte("a\tbc\td")}
	r.updateRender()
	// Render: "a"(0) tab->1..7 "b"(8) "c"(9) tab->10..15 "d"(16)
	cases := []struct{ cx, rx int }{
		{0, 0}, {1, 1}, {2, 8}, {3, 9}, {4, 10}, {5, 16},
	}
	for _, c := range cases {
		if got := r.CxToRx(c.cx); got != c.rx {
			t.Errorf("CxToRx(%d) = %d, want %d", c.cx, got, c.rx)
		}
		// RxToCx should round-trip on exact column boundaries.
		if got := r.RxToCx(c.rx); got != c.cx {
			t.Errorf("RxToCx(%d) = %d, want %d", c.rx, got, c.cx)
		}
	}
}

func TestRxToCxInsideTab(t *testing.T) {
	r := &Row{chars: []byte("a\tb")}
	r.updateRender()
	// rx 3 is inside the tab expansion (cols 1..7); should snap to the tab byte (cx 1).
	if got := r.RxToCx(3); got != 1 {
		t.Fatalf("RxToCx(3) = %d, want 1", got)
	}
}

func TestLoadSaveRoundTripNoTrailingNewline(t *testing.T) {
	in := "alpha\nbeta\ngamma"
	b, err := Load(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if b.NumRows() != 3 {
		t.Fatalf("NumRows = %d, want 3", b.NumRows())
	}
	var out bytes.Buffer
	n, err := b.Save(&out)
	if err != nil {
		t.Fatal(err)
	}
	want := "alpha\nbeta\ngamma\n" // Save always terminates lines
	if out.String() != want {
		t.Fatalf("saved %q, want %q", out.String(), want)
	}
	if n != len(want) {
		t.Fatalf("n = %d, want %d", n, len(want))
	}
}

func TestLoadTrailingNewlineNoDupRow(t *testing.T) {
	b, err := Load(strings.NewReader("one\ntwo\n"))
	if err != nil {
		t.Fatal(err)
	}
	if b.NumRows() != 2 {
		t.Fatalf("NumRows = %d, want 2 (no spurious empty final row)", b.NumRows())
	}
}

func TestLoadCRLF(t *testing.T) {
	b, err := Load(strings.NewReader("win\r\ndows\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if b.NumRows() != 2 {
		t.Fatalf("NumRows = %d, want 2", b.NumRows())
	}
	if b.Row(0).String() != "win" || b.Row(1).String() != "dows" {
		t.Fatalf("rows = %q, %q (CR not stripped)", b.Row(0).String(), b.Row(1).String())
	}
}

func TestLoadEmpty(t *testing.T) {
	b, err := Load(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if b.NumRows() != 0 {
		t.Fatalf("NumRows = %d, want 0", b.NumRows())
	}
}

func TestSaveBinaryPreserved(t *testing.T) {
	raw := "a\x00\x01b"
	b, err := Load(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := b.Save(&out); err != nil {
		t.Fatal(err)
	}
	if out.String() != raw+"\n" {
		t.Fatalf("binary not preserved: %q", out.String())
	}
}

func TestInsertDeleteRow(t *testing.T) {
	b := bufFrom("a", "c")
	b.InsertRow(1, []byte("b"))
	if b.NumRows() != 3 || b.Row(1).String() != "b" {
		t.Fatalf("after insert: %d rows, row1=%q", b.NumRows(), b.Row(1).String())
	}
	b.DeleteRow(1)
	if b.NumRows() != 2 || b.Row(1).String() != "c" {
		t.Fatalf("after delete: %d rows, row1=%q", b.NumRows(), b.Row(1).String())
	}
}
