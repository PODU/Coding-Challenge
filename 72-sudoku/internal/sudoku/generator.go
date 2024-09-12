package sudoku

import "math/rand"

// GenOptions configures Generate.
type GenOptions struct {
	Difficulty Difficulty
	Seed       int64 // the same seed gives the same puzzle
	Symmetric  bool  // clues symmetric under 180-degree rotation
}

// Generate returns a puzzle with exactly one solution, and that solution.
func Generate(opts GenOptions) (puzzle, solution *Board) {
	rng := rand.New(rand.NewSource(opts.Seed))

	// Start from a random solved grid and remove clues in random order, putting
	// back any whose removal would allow a second solution.
	full := buildSolvedGrid(rng)
	solution = full.Clone()
	work := full.Clone()
	target := opts.Difficulty.ClueTarget(rng)

	positions := rng.Perm(81)
	removed := make([]bool, 81)
	clues := 81

	for _, p := range positions {
		if clues <= target {
			break
		}
		if removed[p] {
			continue
		}
		partner := 80 - p // the cell opposite p under 180-degree rotation

		group := []int{p}
		if opts.Symmetric && partner != p && !removed[partner] {
			group = append(group, partner)
		}
		if clues-len(group) < 17 {
			continue // 17 is the minimum for a unique solution
		}

		saved := make([]uint8, len(group))
		for i, idx := range group {
			saved[i] = work.cells[idx]
			work.cells[idx] = 0
		}

		if HasUniqueSolution(work) {
			for _, idx := range group {
				removed[idx] = true
			}
			clues -= len(group)
		} else {
			for i, idx := range group {
				work.cells[idx] = saved[i]
			}
		}
	}

	work.markGivens()
	return work, solution
}

// buildSolvedGrid solves an empty board, trying candidates in random order.
func buildSolvedGrid(rng *rand.Rand) *Board {
	empty := &Board{}
	s := newSolver(empty)
	s.rng = rng
	s.captureFirst = true
	s.limit = 1
	s.search()
	return s.first
}
