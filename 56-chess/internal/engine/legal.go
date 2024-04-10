package engine

// LegalMoves returns the pseudo-legal moves that don't leave the mover in
// check. Trying each move covers pins and check evasions without special cases.
func (b *Board) LegalMoves() []Move {
	us := b.side
	pseudo := b.PseudoLegalMoves()
	legal := make([]Move, 0, len(pseudo))
	for _, m := range pseudo {
		b.MakeMove(m)
		if !b.IsSquareAttacked(b.kings[us], us.Opposite()) {
			legal = append(legal, m)
		}
		b.UnmakeMove()
	}
	return legal
}

// HasLegalMove reports whether the side to move has at least one legal move.
func (b *Board) HasLegalMove() bool {
	us := b.side
	for _, m := range b.PseudoLegalMoves() {
		b.MakeMove(m)
		ok := !b.IsSquareAttacked(b.kings[us], us.Opposite())
		b.UnmakeMove()
		if ok {
			return true
		}
	}
	return false
}
