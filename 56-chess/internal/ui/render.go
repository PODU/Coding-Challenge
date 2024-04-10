// Package ui draws the board and runs the command-line game.
package ui

import (
	"strings"

	"chess/internal/engine"
)

var unicodePieces = map[engine.Piece]rune{
	engine.WKing: '♔', engine.WQueen: '♕', engine.WRook: '♖',
	engine.WBishop: '♗', engine.WKnight: '♘', engine.WPawn: '♙',
	engine.BKing: '♚', engine.BQueen: '♛', engine.BRook: '♜',
	engine.BBishop: '♝', engine.BKnight: '♞', engine.BPawn: '♟',
}

// Render draws the board from White's side, using chess glyphs if unicode is
// set and FEN letters otherwise.
func Render(b *engine.Board, unicode bool) string {
	var sb strings.Builder
	for rank := 7; rank >= 0; rank-- {
		sb.WriteByte(byte('1' + rank))
		sb.WriteByte(' ')
		for file := 0; file < 8; file++ {
			p := b.PieceAt(engine.Sq(file, rank))
			sb.WriteByte(' ')
			if p == engine.Empty {
				sb.WriteByte('.')
			} else if unicode {
				sb.WriteRune(unicodePieces[p])
			} else {
				sb.WriteByte(p.Symbol())
			}
		}
		sb.WriteByte('\n')
	}
	sb.WriteString("   a b c d e f g h\n")
	return sb.String()
}
