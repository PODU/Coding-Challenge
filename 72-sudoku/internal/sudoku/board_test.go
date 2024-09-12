package sudoku

import (
	"strings"
	"testing"
)

const easyPuzzle = "53..7....6..195....98....6.8...6...34..8.3..17...2...6.6....28....419..5....8..79"

func TestParseRoundTrip(t *testing.T) {
	b, err := ParseBoard(easyPuzzle)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := strings.ReplaceAll(easyPuzzle, "0", ".")
	if got := b.String(); got != want {
		t.Errorf("round-trip mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestParseZeroAndDotEquivalent(t *testing.T) {
	withDots := easyPuzzle
	withZeros := strings.ReplaceAll(easyPuzzle, ".", "0")
	a, err := ParseBoard(withDots)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseBoard(withZeros)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Equal(b) {
		t.Error("'.' and '0' parses differ")
	}
}

func TestLenientParse(t *testing.T) {
	art := `
		5 3 . | . 7 . | . . .
		6 . . | 1 9 5 | . . .
		. 9 8 | . . . | . 6 .
		------+-------+------
		8 . . | . 6 . | . . 3
		4 . . | 8 . 3 | . . 1
		7 . . | . 2 . | . . 6
		------+-------+------
		. 6 . | . . . | 2 8 .
		. . . | 4 1 9 | . . 5
		. . . | . 8 . | . 7 9
	`
	got, err := ParseBoard(art)
	if err != nil {
		t.Fatalf("lenient parse: %v", err)
	}
	want, _ := ParseBoard(easyPuzzle)
	if !got.Equal(want) {
		t.Errorf("ASCII-art parse != canonical parse\n got %s\nwant %s", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"too short": strings.Repeat(".", 80),
		"too long":  strings.Repeat(".", 82),
		"bad digit": strings.Repeat(".", 80) + "x",
	}
	for name, in := range cases {
		if _, err := ParseBoard(in); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestCountAndClone(t *testing.T) {
	b, _ := ParseBoard(easyPuzzle)
	if got := b.Count(); got != 30 {
		t.Errorf("Count = %d, want 30", got)
	}
	c := b.Clone()
	c.set(0, 2, 9)
	if b.At(0, 2) != 0 {
		t.Error("Clone is not independent of original")
	}
}

func TestGridRenders(t *testing.T) {
	b, _ := ParseBoard(easyPuzzle)
	g := b.Grid()
	if !strings.Contains(g, "5") || strings.Count(g, "\n") < 9 {
		t.Errorf("Grid output looks wrong:\n%s", g)
	}
}
