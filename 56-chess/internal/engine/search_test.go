package engine

import (
	"math/rand"
	"testing"
)

func TestRandomMoveLegal(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	b := NewBoard()
	for i := 0; i < 40; i++ {
		if b.Status().IsOver() {
			break
		}
		m := RandomMove(b, rng)
		if m.IsNull() {
			t.Fatal("RandomMove returned null in a non-terminal position")
		}
		// Confirm it is among the legal moves.
		ok := false
		for _, lm := range b.LegalMoves() {
			if lm == m {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("RandomMove returned an illegal move %s", m)
		}
		b.MakeMove(m)
	}
}

func TestMateInOne(t *testing.T) {
	cases := []struct {
		fen  string
		mate string
	}{
		// Back-rank mate with the rook.
		{"6k1/5ppp/8/8/8/8/8/R5K1 w - - 0 1", "a1a8"},
		// Queen delivers mate.
		{"6k1/5ppp/8/8/8/8/5PPP/3Q2K1 w - - 0 1", "d1d8"},
	}
	for _, tc := range cases {
		b := mustParse(t, tc.fen)
		m, score := BestMove(b, 2)
		if m.String() != tc.mate {
			t.Errorf("%q: BestMove = %s, want mate %s", tc.fen, m, tc.mate)
		}
		if score < MateScore-10 {
			t.Errorf("%q: mate score = %d, expected near %d", tc.fen, score, MateScore)
		}
		// Verify it is actually mate.
		b.MakeMove(m)
		if b.Status() != Checkmate {
			t.Errorf("%q: move %s did not produce checkmate", tc.fen, m)
		}
	}
}

func TestMateInTwo(t *testing.T) {
	// Morphy's famous mate in two: 1.Ra6 bxa6 2.b7# (or 1...b5 2.Rxa7#).
	b := mustParse(t, "kbK5/pp6/1P6/8/8/8/8/R7 w - - 0 1")
	m, score := BestMove(b, 4)
	if score < MateScore-10 {
		t.Errorf("expected a forced mate, got score %d (move %s)", score, m)
	}
}

func TestAlphaBetaEqualsMinimax(t *testing.T) {
	// On a fixed quiet position, alpha-beta must return the same value as a plain
	// minimax search to the same depth.
	positions := []string{
		StartFEN,
		"r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
	}
	for _, fen := range positions {
		b := mustParse(t, fen)
		_, ab := BestMove(b, 3)
		plain := MinimaxPlain(b, 3)
		if ab != plain {
			t.Errorf("%q: alpha-beta=%d, minimax=%d", fen, ab, plain)
		}
	}
}
