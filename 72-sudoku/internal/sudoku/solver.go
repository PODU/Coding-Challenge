package sudoku

import (
	"math/bits"
	"math/rand"
)

// solver keeps bitmasks of the digits used in each row, column and box, so
// checking a candidate is a couple of bit operations.
type solver struct {
	b            *Board
	rows         [9]uint16
	cols         [9]uint16
	boxes        [9]uint16
	rng          *rand.Rand // if set, candidates are tried in random order
	count        int
	limit        int
	captureFirst bool
	first        *Board
	captured     bool
}

const allDigits uint16 = 0x3FE // bits 1..9 set

// newSolver works on a copy of b. It returns nil if b already breaks the rules.
func newSolver(b *Board) *solver {
	s := &solver{b: b.Clone()}
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			v := s.b.at(r, c)
			if v == 0 {
				continue
			}
			m := bit(v)
			bi := (r/3)*3 + c/3
			if s.rows[r]&m != 0 || s.cols[c]&m != 0 || s.boxes[bi]&m != 0 {
				return nil
			}
			s.rows[r] |= m
			s.cols[c] |= m
			s.boxes[bi] |= m
		}
	}
	return s
}

// candidates returns the digits that can go at (r, c), as a bitmask.
func (s *solver) candidates(r, c int) uint16 {
	bi := (r/3)*3 + c/3
	return allDigits &^ (s.rows[r] | s.cols[c] | s.boxes[bi])
}

func (s *solver) place(r, c int, v uint8) {
	s.b.set(r, c, v)
	m := bit(v)
	s.rows[r] |= m
	s.cols[c] |= m
	s.boxes[(r/3)*3+c/3] |= m
}

func (s *solver) unplace(r, c int, v uint8) {
	s.b.set(r, c, 0)
	m := bit(v)
	s.rows[r] &^= m
	s.cols[c] &^= m
	s.boxes[(r/3)*3+c/3] &^= m
}

// selectCell picks the empty cell with the fewest candidates, which keeps the
// search tree small. ok is false when the board is full; found0 means some
// empty cell has no candidates, so this branch is dead.
func (s *solver) selectCell() (br, bc int, mask uint16, ok, dead bool) {
	best := 10
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if s.b.at(r, c) != 0 {
				continue
			}
			cand := s.candidates(r, c)
			n := popcount(cand)
			if n == 0 {
				return r, c, 0, true, true
			}
			if n < best {
				best, br, bc, mask, ok = n, r, c, cand, true
				if n == 1 {
					return
				}
			}
		}
	}
	return
}

// search fills the board recursively, stopping once s.limit solutions are found.
func (s *solver) search() {
	if s.limit > 0 && s.count >= s.limit {
		return
	}
	r, c, mask, ok, dead := s.selectCell()
	if !ok {
		s.count++
		if s.captureFirst && !s.captured {
			s.first = s.b.Clone()
			s.captured = true
		}
		return
	}
	if dead {
		return
	}
	order := digitsFromMask(mask)
	if s.rng != nil {
		s.rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	}
	for _, v := range order {
		s.place(r, c, v)
		s.search()
		s.unplace(r, c, v)
		if s.limit > 0 && s.count >= s.limit {
			return
		}
	}
}

// Solve returns a solution, or false if there is none. b is not modified.
func Solve(b *Board) (*Board, bool) {
	s := newSolver(b)
	if s == nil {
		return nil, false
	}
	s.captureFirst = true
	s.limit = 1
	s.search()
	if s.captured {
		return s.first, true
	}
	return nil, false
}

// CountSolutions counts solutions, stopping at limit. limit <= 0 counts them
// all.
func CountSolutions(b *Board, limit int) int {
	s := newSolver(b)
	if s == nil {
		return 0
	}
	s.limit = limit
	s.search()
	return s.count
}

// HasUniqueSolution reports whether b has exactly one solution.
func HasUniqueSolution(b *Board) bool {
	return CountSolutions(b, 2) == 1
}

func popcount(x uint16) int { return bits.OnesCount16(x) }

// digitsFromMask lists the digits in mask, in ascending order.
func digitsFromMask(mask uint16) []uint8 {
	out := make([]uint8, 0, 9)
	for v := uint8(1); v <= 9; v++ {
		if mask&bit(v) != 0 {
			out = append(out, v)
		}
	}
	return out
}
