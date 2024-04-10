package engine

import "testing"

func mustParse(t *testing.T, fen string) *Board {
	t.Helper()
	b, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN(%q): %v", fen, err)
	}
	return b
}

func TestCheckmateFoolsMate(t *testing.T) {
	// Position after 1.f3 e5 2.g4 Qh4#.
	b := mustParse(t, "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3")
	if b.Status() != Checkmate {
		t.Errorf("Status = %v, want Checkmate", b.Status())
	}
	if !b.InCheck(White) {
		t.Error("white should be in check")
	}
}

func TestCheckmateBackRank(t *testing.T) {
	b := mustParse(t, "6k1/5ppp/8/8/8/8/8/R5K1 w - - 0 1")
	// White to move and mate with Ra8.
	m, ok := ParseMove(b, "a1a8")
	if !ok {
		t.Fatal("a1a8 should be legal")
	}
	b.MakeMove(m)
	if b.Status() != Checkmate {
		t.Errorf("after Ra8, Status = %v, want Checkmate", b.Status())
	}
}

func TestCheckmateSmothered(t *testing.T) {
	// Smothered mate delivered: knight on f7 mates the king on h8.
	b := mustParse(t, "6rk/5Npp/8/8/8/8/8/6K1 b - - 0 1")
	if b.Status() != Checkmate {
		t.Errorf("Status = %v, want Checkmate", b.Status())
	}
}

func TestStalemate(t *testing.T) {
	// Classic K+Q stalemate: black king a8, white king c7... use known FEN.
	b := mustParse(t, "k7/8/1Q6/8/8/8/8/7K b - - 0 1")
	if b.Status() != Stalemate {
		t.Errorf("Status = %v, want Stalemate", b.Status())
	}
	if b.InCheck(Black) {
		t.Error("black should not be in check in stalemate")
	}
	if len(b.LegalMoves()) != 0 {
		t.Error("stalemate must have no legal moves")
	}
}

func TestFiftyMoveDraw(t *testing.T) {
	b := mustParse(t, "8/8/8/4k3/8/8/4K3/8 w - - 100 80")
	if b.Status() != DrawFiftyMove {
		t.Errorf("Status = %v, want DrawFiftyMove", b.Status())
	}
}

func TestInsufficientMaterial(t *testing.T) {
	draws := []string{
		"8/8/8/4k3/8/8/4K3/8 w - - 0 1",     // K vs K
		"8/8/8/4k3/8/5N2/4K3/8 w - - 0 1",   // K+N vs K
		"8/8/8/4k3/8/5B2/4K3/8 w - - 0 1",   // K+B vs K
		"8/8/2b5/4k3/8/5B2/4K3/8 w - - 0 1", // K+B vs K+B same color (c6 dark, f3 dark)
	}
	for _, fen := range draws {
		b := mustParse(t, fen)
		if !b.InsufficientMaterial() {
			t.Errorf("%q: expected insufficient material", fen)
		}
	}
	notDraws := []string{
		"8/8/8/4k3/8/4P3/4K3/8 w - - 0 1",     // pawn present
		"8/8/8/4k3/8/4R3/4K3/8 w - - 0 1",     // rook present
		"8/5b2/2b5/4k3/8/5B2/4K3/8 w - - 0 1", // bishops on both colors
	}
	for _, fen := range notDraws {
		b := mustParse(t, fen)
		if b.InsufficientMaterial() {
			t.Errorf("%q: expected sufficient material", fen)
		}
	}
}

func TestThreefoldRepetition(t *testing.T) {
	b := NewBoard()
	// Shuffle knights back and forth to repeat the start position three times.
	seq := []string{"g1f3", "g8f6", "f3g1", "f6g8"}
	for rep := 0; rep < 2; rep++ {
		for _, mv := range seq {
			m, ok := ParseMove(b, mv)
			if !ok {
				t.Fatalf("move %s illegal", mv)
			}
			b.MakeMove(m)
		}
	}
	if !b.IsThreefoldRepetition() {
		t.Error("expected threefold repetition of the start position")
	}
	if b.Status() != DrawRepetition {
		t.Errorf("Status = %v, want DrawRepetition", b.Status())
	}
}
