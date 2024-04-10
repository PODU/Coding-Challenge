package engine

// MoveFlag marks moves that need special handling.
type MoveFlag uint8

const (
	FlagNormal      MoveFlag = 0
	FlagDoublePush  MoveFlag = 1 // sets the en-passant square
	FlagEnPassant   MoveFlag = 2
	FlagCastleKing  MoveFlag = 3
	FlagCastleQueen MoveFlag = 4
)

// Move is a move. Promotion is NoType unless the move promotes.
type Move struct {
	From      Square
	To        Square
	Promotion PieceType
	Flag      MoveFlag
}

// NullMove means "no move".
var NullMove = Move{From: NoSquare, To: NoSquare}

// IsNull reports whether m is NullMove.
func (m Move) IsNull() bool { return m == NullMove }

// String returns the move in coordinate notation, such as "e2e4" or "e7e8q".
func (m Move) String() string {
	if m.From == NoSquare && m.To == NoSquare {
		return "0000"
	}
	s := m.From.String() + m.To.String()
	if m.Promotion != NoType {
		s += string(promoLetter(m.Promotion))
	}
	return s
}

func promoLetter(t PieceType) byte {
	switch t {
	case Queen:
		return 'q'
	case Rook:
		return 'r'
	case Bishop:
		return 'b'
	case Knight:
		return 'n'
	}
	return '?'
}

// ParseMove looks up a coordinate-notation move such as "e2e4" among the legal
// moves of b, which also fills in its flags. It returns false if the text is
// malformed or the move is illegal.
func ParseMove(b *Board, s string) (Move, bool) {
	if len(s) != 4 && len(s) != 5 {
		return NullMove, false
	}
	from, ok1 := ParseSquare(s[0:2])
	to, ok2 := ParseSquare(s[2:4])
	if !ok1 || !ok2 {
		return NullMove, false
	}
	var promo PieceType = NoType
	if len(s) == 5 {
		switch s[4] {
		case 'q', 'Q':
			promo = Queen
		case 'r', 'R':
			promo = Rook
		case 'b', 'B':
			promo = Bishop
		case 'n', 'N':
			promo = Knight
		default:
			return NullMove, false
		}
	}
	for _, m := range b.LegalMoves() {
		if m.From == from && m.To == to && m.Promotion == promo {
			return m, true
		}
	}
	return NullMove, false
}
