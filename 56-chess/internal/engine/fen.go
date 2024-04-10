package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// StartFEN is the standard starting position.
const StartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// ParseFEN parses a FEN string. Each rank must add up to eight squares and each
// side must have exactly one king. The halfmove clock and fullmove number may
// be omitted.
func ParseFEN(fen string) (*Board, error) {
	fields := strings.Fields(fen)
	if len(fields) < 4 {
		return nil, fmt.Errorf("fen: expected at least 4 fields, got %d", len(fields))
	}

	b := &Board{ep: NoSquare, kings: [2]Square{NoSquare, NoSquare}}

	// Field 1: piece placement (rank 8 first).
	ranks := strings.Split(fields[0], "/")
	if len(ranks) != 8 {
		return nil, fmt.Errorf("fen: expected 8 ranks, got %d", len(ranks))
	}
	var kingCount [2]int
	for i, rowStr := range ranks {
		rank := 7 - i
		file := 0
		for _, ch := range []byte(rowStr) {
			if ch >= '1' && ch <= '8' {
				file += int(ch - '0')
				if file > 8 {
					return nil, fmt.Errorf("fen: rank %d overflows", rank+1)
				}
				continue
			}
			pc, ok := pieceFromChar(ch)
			if !ok {
				return nil, fmt.Errorf("fen: illegal piece char %q", string(ch))
			}
			if file >= 8 {
				return nil, fmt.Errorf("fen: rank %d overflows", rank+1)
			}
			sq := Sq(file, rank)
			b.squares[sq] = pc
			if pc.Type() == King {
				kingCount[pc.Color()]++
				b.kings[pc.Color()] = sq
			}
			file++
		}
		if file != 8 {
			return nil, fmt.Errorf("fen: rank %d does not sum to 8", rank+1)
		}
	}
	if kingCount[White] != 1 || kingCount[Black] != 1 {
		return nil, errors.New("fen: must have exactly one king per side")
	}

	// Field 2: side to move.
	switch fields[1] {
	case "w":
		b.side = White
	case "b":
		b.side = Black
	default:
		return nil, fmt.Errorf("fen: illegal side to move %q", fields[1])
	}

	// Field 3: castling rights.
	if fields[2] != "-" {
		for _, ch := range []byte(fields[2]) {
			switch ch {
			case 'K':
				b.castling |= CastleWK
			case 'Q':
				b.castling |= CastleWQ
			case 'k':
				b.castling |= CastleBK
			case 'q':
				b.castling |= CastleBQ
			default:
				return nil, fmt.Errorf("fen: illegal castling char %q", string(ch))
			}
		}
	}

	// Field 4: en-passant target.
	if fields[3] != "-" {
		sq, ok := ParseSquare(fields[3])
		if !ok {
			return nil, fmt.Errorf("fen: illegal en-passant square %q", fields[3])
		}
		b.ep = sq
	}

	// Field 5: halfmove clock (optional).
	if len(fields) >= 5 {
		n, err := strconv.Atoi(fields[4])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("fen: illegal halfmove clock %q", fields[4])
		}
		b.halfmove = n
	}

	// Field 6: fullmove number (optional).
	b.fullmove = 1
	if len(fields) >= 6 {
		n, err := strconv.Atoi(fields[5])
		if err != nil || n < 1 {
			return nil, fmt.Errorf("fen: illegal fullmove number %q", fields[5])
		}
		b.fullmove = n
	}

	b.hash = b.computeHash()
	b.hashHistory = append(b.hashHistory, b.hash)
	return b, nil
}

// FEN returns the position as a FEN string. It round-trips with ParseFEN.
func (b *Board) FEN() string {
	var sb strings.Builder

	for rank := 7; rank >= 0; rank-- {
		empty := 0
		for file := 0; file < 8; file++ {
			pc := b.squares[Sq(file, rank)]
			if pc == Empty {
				empty++
				continue
			}
			if empty > 0 {
				sb.WriteByte(byte('0' + empty))
				empty = 0
			}
			sb.WriteByte(pc.Symbol())
		}
		if empty > 0 {
			sb.WriteByte(byte('0' + empty))
		}
		if rank > 0 {
			sb.WriteByte('/')
		}
	}

	sb.WriteByte(' ')
	if b.side == White {
		sb.WriteByte('w')
	} else {
		sb.WriteByte('b')
	}

	sb.WriteByte(' ')
	if b.castling == 0 {
		sb.WriteByte('-')
	} else {
		if b.castling&CastleWK != 0 {
			sb.WriteByte('K')
		}
		if b.castling&CastleWQ != 0 {
			sb.WriteByte('Q')
		}
		if b.castling&CastleBK != 0 {
			sb.WriteByte('k')
		}
		if b.castling&CastleBQ != 0 {
			sb.WriteByte('q')
		}
	}

	sb.WriteByte(' ')
	sb.WriteString(b.ep.String())

	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(b.halfmove))
	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(b.fullmove))

	return sb.String()
}

func pieceFromChar(ch byte) (Piece, bool) {
	var c Color = White
	if ch >= 'a' && ch <= 'z' {
		c = Black
		ch -= 'a' - 'A'
	}
	switch ch {
	case 'P':
		return MakePiece(c, Pawn), true
	case 'N':
		return MakePiece(c, Knight), true
	case 'B':
		return MakePiece(c, Bishop), true
	case 'R':
		return MakePiece(c, Rook), true
	case 'Q':
		return MakePiece(c, Queen), true
	case 'K':
		return MakePiece(c, King), true
	}
	return Empty, false
}
