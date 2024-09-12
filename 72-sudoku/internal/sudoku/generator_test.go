package sudoku

import "testing"

var allDifficulties = []Difficulty{Easy, Medium, Hard, Expert, Extreme}

func TestGenerateUniqueAndSolvable(t *testing.T) {
	for _, d := range allDifficulties {
		t.Run(d.String(), func(t *testing.T) {
			puzzle, solution := Generate(GenOptions{Difficulty: d, Seed: 1})
			if !HasUniqueSolution(puzzle) {
				t.Fatal("generated puzzle is not uniquely solvable")
			}
			if !IsSolved(solution) {
				t.Fatal("returned solution is not a valid solved board")
			}
			got, ok := Solve(puzzle)
			if !ok || !got.Equal(solution) {
				t.Error("solving the puzzle does not yield the returned solution")
			}
		})
	}
}

func TestGenerateClueBand(t *testing.T) {
	for _, d := range allDifficulties {
		band := clueBands[d]
		puzzle, _ := Generate(GenOptions{Difficulty: d, Seed: 7})
		n := puzzle.Count()
		if n < 17 {
			t.Errorf("%s: %d clues is below the 17 minimum", d, n)
		}
		// Carving may stop above the target band if uniqueness blocks removal,
		// but it must never produce fewer than the band's lower bound.
		if n < band[0] {
			t.Errorf("%s: %d clues below band lower bound %d", d, n, band[0])
		}
	}
}

func TestGenerateGivensMarked(t *testing.T) {
	puzzle, _ := Generate(GenOptions{Difficulty: Hard, Seed: 3})
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			filled := puzzle.At(r, c) != 0
			if filled != puzzle.IsGiven(r, c) {
				t.Errorf("(%d,%d): filled=%v but given=%v", r, c, filled, puzzle.IsGiven(r, c))
			}
		}
	}
}

func TestGenerateDeterministic(t *testing.T) {
	p1, s1 := Generate(GenOptions{Difficulty: Hard, Seed: 42})
	p2, s2 := Generate(GenOptions{Difficulty: Hard, Seed: 42})
	if p1.String() != p2.String() {
		t.Errorf("same seed produced different puzzles:\n%s\n%s", p1, p2)
	}
	if s1.String() != s2.String() {
		t.Errorf("same seed produced different solutions:\n%s\n%s", s1, s2)
	}
}

func TestGenerateDifferentSeedsDiffer(t *testing.T) {
	p1, _ := Generate(GenOptions{Difficulty: Hard, Seed: 1})
	p2, _ := Generate(GenOptions{Difficulty: Hard, Seed: 2})
	if p1.String() == p2.String() {
		t.Error("different seeds produced identical puzzles")
	}
}

func TestGenerateSymmetric(t *testing.T) {
	puzzle, _ := Generate(GenOptions{Difficulty: Medium, Seed: 9, Symmetric: true})
	if !HasUniqueSolution(puzzle) {
		t.Fatal("symmetric puzzle is not unique")
	}
	for i := 0; i < 81; i++ {
		filled := puzzle.cells[i] != 0
		partner := puzzle.cells[80-i] != 0
		if filled != partner {
			t.Errorf("position %d breaks 180-degree symmetry", i)
		}
	}
}
