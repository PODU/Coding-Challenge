// Package sudoku has the board, validation, a backtracking solver and a puzzle
// generator.
package sudoku

import (
	"errors"
	"fmt"
	"strings"
)

// Board is a 9x9 grid where 0 means empty. It is a plain array so copying a
// board is just a value copy.
type Board struct {
	cells [81]uint8 // row-major: index = row*9 + col
	given [81]bool  // clues the player can't change
}

// ErrInvalidLength means the input didn't have exactly 81 cells.
var ErrInvalidLength = errors.New("sudoku: board must contain exactly 81 cells")

func index(r, c int) int { return r*9 + c }

func (b *Board) at(r, c int) uint8 { return b.cells[index(r, c)] }

func (b *Board) set(r, c int, v uint8) { b.cells[index(r, c)] = v }

// boxStart returns the top-left cell of the box containing (r, c).
func boxStart(r, c int) (int, int) { return (r / 3) * 3, (c / 3) * 3 }

// At returns the digit at (r, c), 0-indexed; 0 means empty.
func (b *Board) At(r, c int) uint8 { return b.at(r, c) }

// IsGiven reports whether (r, c) is one of the puzzle's clues.
func (b *Board) IsGiven(r, c int) bool { return b.given[index(r, c)] }

// ParseBoard reads 81 cells from s, with '.' or '0' for empty. Whitespace and
// the grid characters '-', '+' and '|' are skipped, so drawn grids work too.
func ParseBoard(s string) (*Board, error) {
	b := &Board{}
	n := 0
	for _, r := range s {
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			continue
		case r == '-' || r == '+' || r == '|' || r == '_':
			continue
		case r == '.' || r == '0':
			if n >= 81 {
				return nil, ErrInvalidLength
			}
			b.cells[n] = 0
			n++
		case r >= '1' && r <= '9':
			if n >= 81 {
				return nil, ErrInvalidLength
			}
			b.cells[n] = uint8(r - '0')
			n++
		default:
			return nil, fmt.Errorf("sudoku: invalid character %q in board", r)
		}
	}
	if n != 81 {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidLength, n)
	}
	return b, nil
}

// String returns the 81-character form that ParseBoard reads.
func (b *Board) String() string {
	var sb strings.Builder
	sb.Grow(81)
	for _, v := range b.cells {
		if v == 0 {
			sb.WriteByte('.')
		} else {
			sb.WriteByte('0' + v)
		}
	}
	return sb.String()
}

// Grid draws the board with row/column numbers and box borders.
func (b *Board) Grid() string {
	var sb strings.Builder
	sb.WriteString("    1 2 3   4 5 6   7 8 9\n")
	sb.WriteString("  +-------+-------+-------+\n")
	for r := 0; r < 9; r++ {
		fmt.Fprintf(&sb, "%d |", r+1)
		for c := 0; c < 9; c++ {
			v := b.at(r, c)
			if v == 0 {
				sb.WriteString(" .")
			} else {
				fmt.Fprintf(&sb, " %d", v)
			}
			if c%3 == 2 {
				sb.WriteString(" |")
			}
		}
		sb.WriteByte('\n')
		if r%3 == 2 {
			sb.WriteString("  +-------+-------+-------+\n")
		}
	}
	return sb.String()
}

// Clone returns a copy of the board.
func (b *Board) Clone() *Board {
	cp := *b
	return &cp
}

// Count returns the number of filled cells.
func (b *Board) Count() int {
	n := 0
	for _, v := range b.cells {
		if v != 0 {
			n++
		}
	}
	return n
}

// Equal compares the digits only, not which cells are clues.
func (b *Board) Equal(o *Board) bool {
	return b.cells == o.cells
}

// markGivens makes every filled cell a clue.
func (b *Board) markGivens() {
	for i, v := range b.cells {
		b.given[i] = v != 0
	}
}
