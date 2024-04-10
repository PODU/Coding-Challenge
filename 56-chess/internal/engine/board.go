package engine

// Castling-rights bits.
const (
	CastleWK uint8 = 1 // White king-side
	CastleWQ uint8 = 2 // White queen-side
	CastleBK uint8 = 4 // Black king-side
	CastleBQ uint8 = 8 // Black queen-side
)

const (
	sqA1 = Square(0)
	sqE1 = Square(4)
	sqH1 = Square(7)
	sqA8 = Square(112)
	sqE8 = Square(116)
	sqH8 = Square(119)
)

// castleSpoil[sq] is ANDed into the castling rights whenever a move starts or
// ends on sq. Moving a king or rook, or capturing a rook at home, clears the
// matching rights.
var castleSpoil = func() [128]uint8 {
	var t [128]uint8
	for i := range t {
		t[i] = 0xF
	}
	t[sqA1] &^= CastleWQ
	t[sqH1] &^= CastleWK
	t[sqE1] &^= CastleWK | CastleWQ
	t[sqA8] &^= CastleBQ
	t[sqH8] &^= CastleBK
	t[sqE8] &^= CastleBK | CastleBQ
	return t
}()

type undo struct {
	move     Move
	captured Piece
	castling uint8
	ep       Square
	halfmove int
	fullmove int
	hash     uint64
	kings    [2]Square
}

// Board is the full game state in a 0x88 representation.
type Board struct {
	squares  [128]Piece
	side     Color
	castling uint8
	ep       Square // en-passant target, or NoSquare
	halfmove int    // plies since the last capture or pawn move
	fullmove int
	kings    [2]Square
	hash     uint64

	history     []undo
	hashHistory []uint64 // for threefold repetition
}

// NewBoard returns the standard chess starting position.
func NewBoard() *Board {
	b, _ := ParseFEN(StartFEN)
	return b
}

// PieceAt returns the piece on sq, or Empty for an empty or off-board square.
func (b *Board) PieceAt(sq Square) Piece {
	if !sq.OnBoard() {
		return Empty
	}
	return b.squares[sq]
}

// SideToMove returns the color to move.
func (b *Board) SideToMove() Color { return b.side }

// CastlingRights returns the current castling-rights bitmask.
func (b *Board) CastlingRights() uint8 { return b.castling }

// EnPassantSquare returns the current en-passant target (NoSquare if none).
func (b *Board) EnPassantSquare() Square { return b.ep }

// HalfmoveClock returns the fifty-move counter (in plies).
func (b *Board) HalfmoveClock() int { return b.halfmove }

// FullmoveNumber returns the fullmove number.
func (b *Board) FullmoveNumber() int { return b.fullmove }

// Hash returns the Zobrist hash of the position.
func (b *Board) Hash() uint64 { return b.hash }

// KingSquare returns the square of the given color's king.
func (b *Board) KingSquare(c Color) Square { return b.kings[c] }

// HasHistory reports whether at least one move can be unmade.
func (b *Board) HasHistory() bool { return len(b.history) > 0 }

// Clone returns a copy of the board with its own history.
func (b *Board) Clone() *Board {
	nb := *b
	nb.history = append([]undo(nil), b.history...)
	nb.hashHistory = append([]uint64(nil), b.hashHistory...)
	return &nb
}

// MakeMove plays m, which must be pseudo-legal in this position. It does not
// check whether the move leaves the mover in check.
func (b *Board) MakeMove(m Move) {
	from, to := m.From, m.To
	pc := b.squares[from]
	c := pc.Color()
	pt := pc.Type()

	u := undo{
		move:     m,
		captured: Empty,
		castling: b.castling,
		ep:       b.ep,
		halfmove: b.halfmove,
		fullmove: b.fullmove,
		hash:     b.hash,
		kings:    b.kings,
	}

	// Take the old en-passant and castling state out of the hash.
	if b.ep != NoSquare {
		b.hash ^= zEnPas[b.ep.File()]
	}
	b.hash ^= zCastling[b.castling]

	resetClock := pt == Pawn
	b.halfmove++

	b.hash ^= zPiece[pc][from]
	b.squares[from] = Empty

	if m.Flag == FlagEnPassant {
		capSq := to - 16
		if c == Black {
			capSq = to + 16
		}
		capPc := b.squares[capSq]
		u.captured = capPc
		b.hash ^= zPiece[capPc][capSq]
		b.squares[capSq] = Empty
		resetClock = true
	} else if b.squares[to] != Empty {
		capPc := b.squares[to]
		u.captured = capPc
		b.hash ^= zPiece[capPc][to]
		resetClock = true
	}

	placed := pc
	if m.Promotion != NoType {
		placed = MakePiece(c, m.Promotion)
	}
	b.squares[to] = placed
	b.hash ^= zPiece[placed][to]

	if rFrom, rTo, ok := castleRook(m); ok {
		rook := b.squares[rFrom]
		b.hash ^= zPiece[rook][rFrom] ^ zPiece[rook][rTo]
		b.squares[rFrom] = Empty
		b.squares[rTo] = rook
	}

	if pt == King {
		b.kings[c] = to
	}

	if resetClock {
		b.halfmove = 0
	}

	b.castling &= castleSpoil[from]
	b.castling &= castleSpoil[to]
	b.hash ^= zCastling[b.castling]

	if m.Flag == FlagDoublePush {
		b.ep = (from + to) / 2
		b.hash ^= zEnPas[b.ep.File()]
	} else {
		b.ep = NoSquare
	}

	if c == Black {
		b.fullmove++
	}
	b.side = c.Opposite()
	b.hash ^= zSide

	b.history = append(b.history, u)
	b.hashHistory = append(b.hashHistory, b.hash)
}

// UnmakeMove reverses the most recent MakeMove.
func (b *Board) UnmakeMove() {
	n := len(b.history) - 1
	u := b.history[n]
	b.history = b.history[:n]
	b.hashHistory = b.hashHistory[:len(b.hashHistory)-1]

	m := u.move
	from, to := m.From, m.To
	moved := b.squares[to]
	c := moved.Color()

	if m.Promotion != NoType {
		moved = MakePiece(c, Pawn)
	}
	b.squares[from] = moved
	b.squares[to] = Empty

	switch {
	case m.Flag == FlagEnPassant:
		capSq := to - 16
		if c == Black {
			capSq = to + 16
		}
		b.squares[capSq] = u.captured
	case u.captured != Empty:
		b.squares[to] = u.captured
	}

	if rFrom, rTo, ok := castleRook(m); ok {
		b.squares[rFrom] = b.squares[rTo]
		b.squares[rTo] = Empty
	}

	b.side = c
	b.castling = u.castling
	b.ep = u.ep
	b.halfmove = u.halfmove
	b.fullmove = u.fullmove
	b.hash = u.hash
	b.kings = u.kings
}

// castleRook returns where the rook starts and ends for a castling move.
func castleRook(m Move) (from, to Square, ok bool) {
	switch m.Flag {
	case FlagCastleKing:
		return m.From + 3, m.From + 1, true
	case FlagCastleQueen:
		return m.From - 4, m.From - 1, true
	}
	return 0, 0, false
}
