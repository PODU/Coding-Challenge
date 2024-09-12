package sudoku

import (
	"strings"
	"testing"
)

// newPlayGame builds a Game over easyPuzzle with givens locked and solvedEasy as
// the known solution.
func newPlayGame(t *testing.T, script string) (*Game, *strings.Builder) {
	t.Helper()
	b := mustParse(t, easyPuzzle)
	b.markGivens()
	out := &strings.Builder{}
	g := NewGame(b, mustParse(t, solvedEasy), strings.NewReader(script), out)
	return g, out
}

func TestPlayLegalPlacement(t *testing.T) {
	// (0,3) is empty in easyPuzzle; 6 is legal there.
	g, _ := newPlayGame(t, "1 4 6\nquit\n")
	if err := g.Run(); err != nil {
		t.Fatal(err)
	}
	if g.Board.At(0, 3) != 6 {
		t.Errorf("expected 6 at (0,3), got %d", g.Board.At(0, 3))
	}
}

func TestPlayIllegalRejected(t *testing.T) {
	// 5 already exists in row 0, so it must be rejected at (0,3).
	g, out := newPlayGame(t, "1 4 5\nquit\n")
	g.Run()
	if g.Board.At(0, 3) != 0 {
		t.Error("illegal placement should not modify the cell")
	}
	if !strings.Contains(out.String(), "conflicts") {
		t.Error("expected a conflict message")
	}
}

func TestPlayGivenLocked(t *testing.T) {
	// (0,0) is a given (5). Attempt to overwrite it.
	g, out := newPlayGame(t, "1 1 4\nquit\n")
	g.Run()
	if g.Board.At(0, 0) != 5 {
		t.Error("given cell must not change")
	}
	if !strings.Contains(out.String(), "given") {
		t.Error("expected a 'given clue' rejection message")
	}
}

func TestPlayClear(t *testing.T) {
	// Place then clear a user cell.
	g, _ := newPlayGame(t, "1 4 6\nclear 1 4\nquit\n")
	g.Run()
	if g.Board.At(0, 3) != 0 {
		t.Error("clear should empty the user cell")
	}
	// Clearing a given must be rejected.
	g2, out := newPlayGame(t, "clear 1 1\nquit\n")
	g2.Run()
	if g2.Board.At(0, 0) != 5 {
		t.Error("clearing a given must not change it")
	}
	if !strings.Contains(out.String(), "given") {
		t.Error("expected rejection clearing a given")
	}
}

func TestPlayCheckSuccess(t *testing.T) {
	b := mustParse(t, easyPuzzle)
	b.markGivens()
	out := &strings.Builder{}
	g := NewGame(b, mustParse(t, solvedEasy), strings.NewReader("check\nquit\n"), out)
	// Fill all non-given cells correctly.
	sol := mustParse(t, solvedEasy)
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if !b.IsGiven(r, c) {
				b.set(r, c, sol.At(r, c))
			}
		}
	}
	g.Run()
	if !strings.Contains(out.String(), "win") && !strings.Contains(out.String(), "Solved") {
		t.Errorf("expected a win message, got:\n%s", out.String())
	}
}

func TestPlayCheckFailure(t *testing.T) {
	b := mustParse(t, easyPuzzle)
	b.markGivens()
	out := &strings.Builder{}
	g := NewGame(b, mustParse(t, solvedEasy), strings.NewReader("check\nquit\n"), out)
	// Fill all non-given cells but flip two to make it full-yet-wrong while legal
	// is hard; instead make it full but incorrect by filling wrong where legal.
	sol := mustParse(t, solvedEasy)
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if !b.IsGiven(r, c) {
				b.set(r, c, sol.At(r, c))
			}
		}
	}
	// Introduce a conflict so the full board is not legal.
	// Find two user cells in the same row and set them equal.
	b.set(2, 0, 9)
	b.set(2, 4, 9) // row 2 now has duplicate 9 (both non-given in easyPuzzle)
	g.Run()
	o := out.String()
	if strings.Contains(o, "win") || strings.Contains(o, "Solved") {
		t.Errorf("wrong board should not report a win:\n%s", o)
	}
}
