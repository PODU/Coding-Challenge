package sudoku

import "testing"

// hard17 is a canonical 17-clue Sudoku (the minimum possible for a unique
// solution) used to exercise the solver's MRV-driven search.
const hard17 = "000000010400000000020000000000050407008000300001090000300400200050100000000806000"

func TestSolveKnownPuzzle(t *testing.T) {
	b := mustParse(t, easyPuzzle)
	sol, ok := Solve(b)
	if !ok {
		t.Fatal("easy puzzle reported unsolvable")
	}
	if sol.String() != mustParse(t, solvedEasy).String() {
		t.Errorf("solution mismatch:\n got %s\nwant %s", sol, solvedEasy)
	}
	// Input must be unchanged.
	if b.String() != mustParse(t, easyPuzzle).String() {
		t.Error("Solve mutated its input")
	}
}

func TestSolveHard17Clue(t *testing.T) {
	b := mustParse(t, hard17)
	if b.Count() != 17 {
		t.Fatalf("fixture has %d clues, want 17", b.Count())
	}
	sol, ok := Solve(b)
	if !ok {
		t.Fatal("17-clue puzzle reported unsolvable")
	}
	if !IsSolved(sol) {
		t.Error("returned solution is not a valid solved board")
	}
	// Solution must agree with all given clues.
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if v := b.At(r, c); v != 0 && sol.At(r, c) != v {
				t.Errorf("solution overwrote given clue at (%d,%d)", r, c)
			}
		}
	}
	if !HasUniqueSolution(b) {
		t.Error("canonical 17-clue puzzle should have a unique solution")
	}
}

func TestNoSolution(t *testing.T) {
	// Legal partial board with no completion: row 0 holds 1..8, so (0,8) needs
	// 9, but column 8 already has a 9 at (1,8).
	board := "12345678." + "........9" + repeat(".", 63)
	b := mustParse(t, board)
	if !IsLegal(b) {
		t.Fatal("fixture should be a legal partial board")
	}
	if _, ok := Solve(b); ok {
		t.Error("Solve should report no solution")
	}
	if n := CountSolutions(b, 2); n != 0 {
		t.Errorf("CountSolutions = %d, want 0", n)
	}
}

func TestIllegalInputUnsolvable(t *testing.T) {
	b := mustParse(t, "11"+repeat(".", 79))
	if _, ok := Solve(b); ok {
		t.Error("illegal board should be unsolvable")
	}
	if CountSolutions(b, 2) != 0 {
		t.Error("illegal board should have 0 solutions")
	}
}

func TestMultipleSolutions(t *testing.T) {
	empty := &Board{}
	if n := CountSolutions(empty, 2); n != 2 {
		t.Errorf("empty board CountSolutions(.,2) = %d, want 2", n)
	}
	if HasUniqueSolution(empty) {
		t.Error("empty board should not be unique")
	}
	// Solver still returns one full solution for an empty board.
	sol, ok := Solve(empty)
	if !ok || !IsSolved(sol) {
		t.Error("Solve should fill an empty board with a valid solution")
	}
}

func TestUniqueSolutionEasy(t *testing.T) {
	if !HasUniqueSolution(mustParse(t, easyPuzzle)) {
		t.Error("easy puzzle should have a unique solution")
	}
}

func BenchmarkSolveHard(b *testing.B) {
	board, _ := ParseBoard(hard17)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := Solve(board); !ok {
			b.Fatal("unsolvable")
		}
	}
}
