package engine

// PseudoLegalMoves returns the moves for the side to move without checking
// whether they leave the king in check; LegalMoves does that. Castling is
// fully checked here, since the transit squares matter too.
func (b *Board) PseudoLegalMoves() []Move {
	moves := make([]Move, 0, 48)
	us := b.side
	for sq := Square(0); sq < 128; sq++ {
		if !sq.OnBoard() {
			sq += 7 // skip the off-board half of the 0x88 rank
			continue
		}
		p := b.squares[sq]
		if p == Empty || p.Color() != us {
			continue
		}
		switch p.Type() {
		case Pawn:
			b.genPawn(sq, us, &moves)
		case Knight:
			b.genStep(sq, us, knightOffsets[:], &moves)
		case Bishop:
			b.genSlide(sq, us, bishopOffsets[:], &moves)
		case Rook:
			b.genSlide(sq, us, rookOffsets[:], &moves)
		case Queen:
			b.genSlide(sq, us, kingOffsets[:], &moves)
		case King:
			b.genStep(sq, us, kingOffsets[:], &moves)
			b.genCastle(sq, us, &moves)
		}
	}
	return moves
}

func (b *Board) genStep(from Square, us Color, offsets []Square, moves *[]Move) {
	for _, off := range offsets {
		to := from + off
		if !to.OnBoard() {
			continue
		}
		t := b.squares[to]
		if t == Empty || t.Color() != us {
			*moves = append(*moves, Move{From: from, To: to})
		}
	}
}

func (b *Board) genSlide(from Square, us Color, offsets []Square, moves *[]Move) {
	for _, off := range offsets {
		for to := from + off; to.OnBoard(); to += off {
			t := b.squares[to]
			if t == Empty {
				*moves = append(*moves, Move{From: from, To: to})
				continue
			}
			if t.Color() != us {
				*moves = append(*moves, Move{From: from, To: to})
			}
			break
		}
	}
}

func (b *Board) genPawn(from Square, us Color, moves *[]Move) {
	var forward Square = 16
	startRank, promoRank := 1, 7
	capOffsets := [2]Square{15, 17}
	if us == Black {
		forward = -16
		startRank, promoRank = 6, 0
		capOffsets = [2]Square{-15, -17}
	}

	one := from + forward
	if one.OnBoard() && b.squares[one] == Empty {
		b.addPawnAdvance(from, one, promoRank, moves)
		if from.Rank() == startRank {
			two := one + forward
			if b.squares[two] == Empty {
				*moves = append(*moves, Move{From: from, To: two, Flag: FlagDoublePush})
			}
		}
	}

	for _, off := range capOffsets {
		to := from + off
		if !to.OnBoard() {
			continue
		}
		t := b.squares[to]
		if t != Empty && t.Color() != us {
			b.addPawnAdvance(from, to, promoRank, moves)
		} else if to == b.ep {
			*moves = append(*moves, Move{From: from, To: to, Flag: FlagEnPassant})
		}
	}
}

func (b *Board) addPawnAdvance(from, to Square, promoRank int, moves *[]Move) {
	if to.Rank() == promoRank {
		for _, pt := range [4]PieceType{Queen, Rook, Bishop, Knight} {
			*moves = append(*moves, Move{From: from, To: to, Promotion: pt})
		}
		return
	}
	*moves = append(*moves, Move{From: from, To: to})
}

func (b *Board) genCastle(from Square, us Color, moves *[]Move) {
	them := us.Opposite()
	// Cannot castle out of check.
	if b.IsSquareAttacked(from, them) {
		return
	}
	if us == White {
		if b.castling&CastleWK != 0 &&
			b.squares[sqE1+1] == Empty && b.squares[sqE1+2] == Empty &&
			!b.IsSquareAttacked(sqE1+1, them) && !b.IsSquareAttacked(sqE1+2, them) {
			*moves = append(*moves, Move{From: sqE1, To: sqE1 + 2, Flag: FlagCastleKing})
		}
		if b.castling&CastleWQ != 0 &&
			b.squares[sqE1-1] == Empty && b.squares[sqE1-2] == Empty && b.squares[sqE1-3] == Empty &&
			!b.IsSquareAttacked(sqE1-1, them) && !b.IsSquareAttacked(sqE1-2, them) {
			*moves = append(*moves, Move{From: sqE1, To: sqE1 - 2, Flag: FlagCastleQueen})
		}
	} else {
		if b.castling&CastleBK != 0 &&
			b.squares[sqE8+1] == Empty && b.squares[sqE8+2] == Empty &&
			!b.IsSquareAttacked(sqE8+1, them) && !b.IsSquareAttacked(sqE8+2, them) {
			*moves = append(*moves, Move{From: sqE8, To: sqE8 + 2, Flag: FlagCastleKing})
		}
		if b.castling&CastleBQ != 0 &&
			b.squares[sqE8-1] == Empty && b.squares[sqE8-2] == Empty && b.squares[sqE8-3] == Empty &&
			!b.IsSquareAttacked(sqE8-1, them) && !b.IsSquareAttacked(sqE8-2, them) {
			*moves = append(*moves, Move{From: sqE8, To: sqE8 - 2, Flag: FlagCastleQueen})
		}
	}
}
