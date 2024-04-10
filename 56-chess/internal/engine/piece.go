// Package engine implements the rules of chess on a 0x88 board, FEN, perft and
// an alpha-beta search. It does no I/O.
package engine

// Color is White (0) or Black (1).
type Color uint8

const (
	White Color = 0
	Black Color = 1
)

// Opposite returns the other color.
func (c Color) Opposite() Color { return c ^ 1 }

func (c Color) String() string {
	if c == White {
		return "white"
	}
	return "black"
}

// PieceType is a kind of piece. NoType means an empty square.
type PieceType uint8

const (
	NoType PieceType = 0
	Pawn   PieceType = 1
	Knight PieceType = 2
	Bishop PieceType = 3
	Rook   PieceType = 4
	Queen  PieceType = 5
	King   PieceType = 6
)

// Piece packs the PieceType into bits 0-2 and the Color into bit 3.
type Piece uint8

const (
	Empty Piece = 0

	WPawn   Piece = Piece(Pawn)
	WKnight Piece = Piece(Knight)
	WBishop Piece = Piece(Bishop)
	WRook   Piece = Piece(Rook)
	WQueen  Piece = Piece(Queen)
	WKing   Piece = Piece(King)

	BPawn   Piece = Piece(Pawn) | 8
	BKnight Piece = Piece(Knight) | 8
	BBishop Piece = Piece(Bishop) | 8
	BRook   Piece = Piece(Rook) | 8
	BQueen  Piece = Piece(Queen) | 8
	BKing   Piece = Piece(King) | 8
)

// MakePiece returns the piece of type t and color c.
func MakePiece(c Color, t PieceType) Piece {
	return Piece(uint8(t) | uint8(c)<<3)
}

// Type returns NoType for Empty.
func (p Piece) Type() PieceType { return PieceType(p & 7) }

// Color is meaningless for Empty.
func (p Piece) Color() Color { return Color(p >> 3) }

// IsEmpty reports whether p is Empty.
func (p Piece) IsEmpty() bool { return p == Empty }

// Symbol returns the FEN letter for p, uppercase for White, or a space for
// Empty.
func (p Piece) Symbol() byte {
	letters := [...]byte{0, 'P', 'N', 'B', 'R', 'Q', 'K'}
	t := p.Type()
	if t == NoType {
		return ' '
	}
	b := letters[t]
	if p.Color() == Black {
		b += 'a' - 'A'
	}
	return b
}

// Square is an index into the 0x88 board. Squares with sq&0x88 != 0 are off
// the board.
type Square int

// NoSquare means no square, e.g. no en-passant target.
const NoSquare Square = -1

// Sq returns the square for a 0-based file and rank.
func Sq(file, rank int) Square { return Square(rank*16 + file) }

// File returns the 0-based file.
func (s Square) File() int { return int(s) & 7 }

// Rank returns the 0-based rank.
func (s Square) Rank() int { return int(s) >> 4 }

// OnBoard reports whether s is a real square.
func (s Square) OnBoard() bool { return s >= 0 && int(s)&0x88 == 0 }

// String returns the square's name, such as "e4", or "-" for NoSquare.
func (s Square) String() string {
	if !s.OnBoard() {
		return "-"
	}
	return string([]byte{byte('a' + s.File()), byte('1' + s.Rank())})
}

// ParseSquare parses a square name such as "e4".
func ParseSquare(name string) (Square, bool) {
	if len(name) != 2 {
		return NoSquare, false
	}
	f := int(name[0] - 'a')
	r := int(name[1] - '1')
	if f < 0 || f > 7 || r < 0 || r > 7 {
		return NoSquare, false
	}
	return Sq(f, r), true
}
