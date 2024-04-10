package engine

// Step offsets on the 0x88 board.
var (
	knightOffsets = [8]Square{33, 31, 18, 14, -33, -31, -18, -14}
	bishopOffsets = [4]Square{15, 17, -15, -17}
	rookOffsets   = [4]Square{16, 1, -16, -1}
	kingOffsets   = [8]Square{16, 1, -16, -1, 15, 17, -15, -17}
)

// IsSquareAttacked reports whether any piece of byColor attacks sq.
func (b *Board) IsSquareAttacked(sq Square, byColor Color) bool {
	if !sq.OnBoard() {
		return false
	}

	// A white pawn on x attacks x+15 and x+17, so look for one on sq-15 or
	// sq-17 (and the mirror image for black).
	if byColor == White {
		if pawnHits(b, sq-15, WPawn) || pawnHits(b, sq-17, WPawn) {
			return true
		}
	} else {
		if pawnHits(b, sq+15, BPawn) || pawnHits(b, sq+17, BPawn) {
			return true
		}
	}

	knight := MakePiece(byColor, Knight)
	for _, off := range knightOffsets {
		t := sq + off
		if t.OnBoard() && b.squares[t] == knight {
			return true
		}
	}

	king := MakePiece(byColor, King)
	for _, off := range kingOffsets {
		t := sq + off
		if t.OnBoard() && b.squares[t] == king {
			return true
		}
	}

	bishop := MakePiece(byColor, Bishop)
	queen := MakePiece(byColor, Queen)
	for _, off := range bishopOffsets {
		for t := sq + off; t.OnBoard(); t += off {
			p := b.squares[t]
			if p == Empty {
				continue
			}
			if p == bishop || p == queen {
				return true
			}
			break
		}
	}

	rook := MakePiece(byColor, Rook)
	for _, off := range rookOffsets {
		for t := sq + off; t.OnBoard(); t += off {
			p := b.squares[t]
			if p == Empty {
				continue
			}
			if p == rook || p == queen {
				return true
			}
			break
		}
	}

	return false
}

func pawnHits(b *Board, from Square, pawn Piece) bool {
	return from.OnBoard() && b.squares[from] == pawn
}

// InCheck reports whether the given color's king is currently attacked.
func (b *Board) InCheck(c Color) bool {
	return b.IsSquareAttacked(b.kings[c], c.Opposite())
}
