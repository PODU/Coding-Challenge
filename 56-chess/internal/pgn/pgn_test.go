package pgn

import (
	"testing"

	"chess/internal/engine"
)

func san(t *testing.T, fen, move string) string {
	t.Helper()
	b, err := engine.ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN(%q): %v", fen, err)
	}
	m, ok := engine.ParseMove(b, move)
	if !ok {
		t.Fatalf("move %q illegal in %q", move, fen)
	}
	return SAN(b, m)
}

func TestSANBasic(t *testing.T) {
	cases := []struct {
		fen, move, want string
	}{
		{engine.StartFEN, "e2e4", "e4"},
		{engine.StartFEN, "g1f3", "Nf3"},
		{"rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 2", "f1c4", "Bc4"},
		// Capture.
		{"rnbqkbnr/ppp1pppp/8/3p4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq d6 0 2", "e4d5", "exd5"},
		// Castling.
		{"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", "O-O"},
		{"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1c1", "O-O-O"},
		// Promotion.
		{"8/P7/8/8/8/8/8/2k4K w - - 0 1", "a7a8q", "a8=Q"},
		// Check.
		{"4k3/8/8/8/8/8/8/4KR2 w - - 0 1", "f1f8", "Rf8+"},
		// Checkmate.
		{"6k1/5ppp/8/8/8/8/8/R5K1 w - - 0 1", "a1a8", "Ra8#"},
	}
	for _, tc := range cases {
		if got := san(t, tc.fen, tc.move); got != tc.want {
			t.Errorf("SAN(%q, %q) = %q, want %q", tc.fen, tc.move, got, tc.want)
		}
	}
}

func TestSANDisambiguation(t *testing.T) {
	// Knights on a3 and e3 can both reach c2, forcing file disambiguation.
	got := san(t, "4k3/8/8/8/8/N3N3/8/4K3 w - - 0 1", "a3c2")
	if got != "Nac2" {
		// Either Nac2 (file) acceptable; ensure file disambiguation present.
		t.Errorf("expected file-disambiguated knight move, got %q", got)
	}
}

func TestExport(t *testing.T) {
	b := engine.NewBoard()
	var moves []engine.Move
	for _, s := range []string{"e2e4", "e7e5", "g1f3", "b8c6"} {
		m, ok := engine.ParseMove(b, s)
		if !ok {
			t.Fatalf("illegal move %s", s)
		}
		moves = append(moves, m)
		b.MakeMove(m)
	}
	want := "1. e4 e5 2. Nf3 Nc6"
	if got := Export(engine.NewBoard(), moves); got != want {
		t.Errorf("Export = %q, want %q", got, want)
	}
}
