package sudoku

func bit(v uint8) uint16 { return 1 << v }

// LegalPlacement reports whether v can go at (r, c) without repeating in its
// row, column or box. Whatever is in (r, c) now is ignored.
func LegalPlacement(b *Board, r, c int, v uint8) bool {
	if v == 0 {
		return true
	}
	for i := 0; i < 9; i++ {
		if i != c && b.at(r, i) == v {
			return false
		}
		if i != r && b.at(i, c) == v {
			return false
		}
	}
	br, bc := boxStart(r, c)
	for dr := 0; dr < 3; dr++ {
		for dc := 0; dc < 3; dc++ {
			rr, cc := br+dr, bc+dc
			if (rr != r || cc != c) && b.at(rr, cc) == v {
				return false
			}
		}
	}
	return true
}

// IsLegal reports whether no row, column or box repeats a digit. Empty cells
// are fine.
func IsLegal(b *Board) bool {
	var rows, cols, boxes [9]uint16
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			v := b.at(r, c)
			if v == 0 {
				continue
			}
			m := bit(v)
			bi := (r/3)*3 + c/3
			if rows[r]&m != 0 || cols[c]&m != 0 || boxes[bi]&m != 0 {
				return false
			}
			rows[r] |= m
			cols[c] |= m
			boxes[bi] |= m
		}
	}
	return true
}

// IsSolved reports whether the board is full and legal.
func IsSolved(b *Board) bool {
	for _, v := range b.cells {
		if v == 0 {
			return false
		}
	}
	return IsLegal(b)
}
