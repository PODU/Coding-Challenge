package engine

// Perft counts the leaf nodes of the legal move tree to the given depth. The
// tests compare it with published counts to check move generation.
func (b *Board) Perft(depth int) uint64 {
	if depth == 0 {
		return 1
	}
	moves := b.PseudoLegalMoves()
	us := b.side
	var nodes uint64
	for _, m := range moves {
		b.MakeMove(m)
		if !b.IsSquareAttacked(b.kings[us], us.Opposite()) {
			if depth == 1 {
				nodes++
			} else {
				nodes += b.Perft(depth - 1)
			}
		}
		b.UnmakeMove()
	}
	return nodes
}

// PerftDivide returns the perft count under each root move, for debugging.
func PerftDivide(b *Board, depth int) map[string]uint64 {
	out := make(map[string]uint64)
	for _, m := range b.LegalMoves() {
		b.MakeMove(m)
		if depth <= 1 {
			out[m.String()] = 1
		} else {
			out[m.String()] = b.Perft(depth - 1)
		}
		b.UnmakeMove()
	}
	return out
}
