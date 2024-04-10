package engine

import "math/rand"

// MateScore is the score of a forced mate. A nearer mate scores slightly higher
// so the search goes for the quickest one.
const MateScore = 1_000_000

const infinity = 1 << 30

// RandomMove returns a random legal move, or NullMove if there are none.
func RandomMove(b *Board, rng *rand.Rand) Move {
	moves := b.LegalMoves()
	if len(moves) == 0 {
		return NullMove
	}
	return moves[rng.Intn(len(moves))]
}

// BestMove runs an alpha-beta negamax search to depth and returns the best
// move and its score in centipawns for the side to move. It returns NullMove if
// there are no legal moves.
func BestMove(b *Board, depth int) (Move, int) {
	if depth < 1 {
		depth = 1
	}
	best := NullMove
	bestScore := -infinity
	alpha, beta := -infinity, infinity
	for _, m := range b.LegalMoves() {
		b.MakeMove(m)
		score := -negamax(b, depth-1, -beta, -alpha, 1)
		b.UnmakeMove()
		if score > bestScore {
			bestScore = score
			best = m
		}
		if score > alpha {
			alpha = score
		}
	}
	return best, bestScore
}

func negamax(b *Board, depth, alpha, beta, ply int) int {
	moves := b.LegalMoves()
	if len(moves) == 0 {
		if b.InCheck(b.side) {
			return -MateScore + ply // mated; prefer slower mates against us
		}
		return 0 // stalemate
	}
	if depth == 0 {
		return b.Evaluate()
	}
	best := -infinity
	for _, m := range moves {
		b.MakeMove(m)
		score := -negamax(b, depth-1, -beta, -alpha, ply+1)
		b.UnmakeMove()
		if score > best {
			best = score
		}
		if best > alpha {
			alpha = best
		}
		if alpha >= beta {
			break // beta cutoff
		}
	}
	return best
}

// MinimaxPlain is negamax without pruning. Tests use it to check that
// alpha-beta gets the same score.
func MinimaxPlain(b *Board, depth int) int {
	return minimaxPlain(b, depth, 0)
}

func minimaxPlain(b *Board, depth, ply int) int {
	moves := b.LegalMoves()
	if len(moves) == 0 {
		if b.InCheck(b.side) {
			return -MateScore + ply
		}
		return 0
	}
	if depth == 0 {
		return b.Evaluate()
	}
	best := -infinity
	for _, m := range moves {
		b.MakeMove(m)
		score := -minimaxPlain(b, depth-1, ply+1)
		b.UnmakeMove()
		if score > best {
			best = score
		}
	}
	return best
}
