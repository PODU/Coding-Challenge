package engine

// Status is whether the game is still going, and if not, why it ended.
type Status int

const (
	Ongoing Status = iota
	Checkmate
	Stalemate
	DrawFiftyMove
	DrawRepetition
	DrawInsufficientMaterial
)

func (s Status) String() string {
	switch s {
	case Ongoing:
		return "ongoing"
	case Checkmate:
		return "checkmate"
	case Stalemate:
		return "stalemate"
	case DrawFiftyMove:
		return "draw (fifty-move rule)"
	case DrawRepetition:
		return "draw (threefold repetition)"
	case DrawInsufficientMaterial:
		return "draw (insufficient material)"
	}
	return "unknown"
}

// IsDraw reports whether the game ended in a draw.
func (s Status) IsDraw() bool {
	return s == Stalemate || s == DrawFiftyMove || s == DrawRepetition || s == DrawInsufficientMaterial
}

// IsOver reports whether the game has ended.
func (s Status) IsOver() bool { return s != Ongoing }

// Status returns the game state. Checkmate and stalemate take precedence over
// the other draw rules.
func (b *Board) Status() Status {
	if !b.HasLegalMove() {
		if b.InCheck(b.side) {
			return Checkmate
		}
		return Stalemate
	}
	if b.halfmove >= 100 {
		return DrawFiftyMove
	}
	if b.IsThreefoldRepetition() {
		return DrawRepetition
	}
	if b.InsufficientMaterial() {
		return DrawInsufficientMaterial
	}
	return Ongoing
}

// IsThreefoldRepetition reports whether this position (by hash) has come up
// at least three times.
func (b *Board) IsThreefoldRepetition() bool {
	count := 0
	for _, h := range b.hashHistory {
		if h == b.hash {
			count++
		}
	}
	return count >= 3
}

// InsufficientMaterial covers K v K, K and one minor piece v K, and positions
// where all the bishops are on the same colour.
func (b *Board) InsufficientMaterial() bool {
	var knights, bishops int
	var lightBishop, darkBishop bool
	for sq := Square(0); sq < 128; sq++ {
		if !sq.OnBoard() {
			sq += 7
			continue
		}
		p := b.squares[sq]
		switch p.Type() {
		case NoType, King:
		case Pawn, Rook, Queen:
			return false
		case Knight:
			knights++
		case Bishop:
			bishops++
			if (sq.File()+sq.Rank())%2 == 0 {
				darkBishop = true
			} else {
				lightBishop = true
			}
		}
	}

	if knights+bishops <= 1 {
		return true
	}
	return knights == 0 && bishops == 2 && !(lightBishop && darkBishop)
}
