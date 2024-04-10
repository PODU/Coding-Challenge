// Package pgn writes moves in SAN and games as PGN movetext.
package pgn

import (
	"fmt"
	"strings"

	"chess/internal/engine"
)

// SAN returns m in Standard Algebraic Notation. b is the position before the
// move.
func SAN(b *engine.Board, m engine.Move) string {
	switch m.Flag {
	case engine.FlagCastleKing:
		return withCheck(b, m, "O-O")
	case engine.FlagCastleQueen:
		return withCheck(b, m, "O-O-O")
	}

	pc := b.PieceAt(m.From)
	pt := pc.Type()
	capture := b.PieceAt(m.To) != engine.Empty || m.Flag == engine.FlagEnPassant

	var sb strings.Builder
	if pt == engine.Pawn {
		if capture {
			sb.WriteByte(byte('a' + m.From.File()))
		}
	} else {
		sb.WriteByte(pieceLetter(pt))
		sb.WriteString(disambiguate(b, m, pt))
	}
	if capture {
		sb.WriteByte('x')
	}
	sb.WriteString(m.To.String())
	if m.Promotion != engine.NoType {
		sb.WriteByte('=')
		sb.WriteByte(pieceLetter(m.Promotion))
	}
	return withCheck(b, m, sb.String())
}

// disambiguate returns the file, rank or square needed when another piece of
// the same type can also reach m.To.
func disambiguate(b *engine.Board, m engine.Move, pt engine.PieceType) string {
	var others []engine.Move
	for _, lm := range b.LegalMoves() {
		if lm.To == m.To && lm.From != m.From && b.PieceAt(lm.From).Type() == pt {
			others = append(others, lm)
		}
	}
	if len(others) == 0 {
		return ""
	}
	sameFile, sameRank := false, false
	for _, o := range others {
		if o.From.File() == m.From.File() {
			sameFile = true
		}
		if o.From.Rank() == m.From.Rank() {
			sameRank = true
		}
	}
	switch {
	case !sameFile:
		return string(rune('a' + m.From.File()))
	case !sameRank:
		return string(rune('1' + m.From.Rank()))
	default:
		return m.From.String()
	}
}

func withCheck(b *engine.Board, m engine.Move, s string) string {
	b.MakeMove(m)
	defer b.UnmakeMove()
	if b.Status() == engine.Checkmate {
		return s + "#"
	}
	if b.InCheck(b.SideToMove()) {
		return s + "+"
	}
	return s
}

func pieceLetter(pt engine.PieceType) byte {
	switch pt {
	case engine.Knight:
		return 'N'
	case engine.Bishop:
		return 'B'
	case engine.Rook:
		return 'R'
	case engine.Queen:
		return 'Q'
	case engine.King:
		return 'K'
	}
	return '?'
}

// Export returns moves as PGN movetext, e.g. "1. e4 e5 2. Nf3 Nc6". start is
// not modified.
func Export(start *engine.Board, moves []engine.Move) string {
	b := start.Clone()
	var sb strings.Builder
	for i, m := range moves {
		if i%2 == 0 {
			fmt.Fprintf(&sb, "%d. ", b.FullmoveNumber())
		}
		sb.WriteString(SAN(b, m))
		sb.WriteByte(' ')
		b.MakeMove(m)
	}
	return strings.TrimSpace(sb.String())
}
