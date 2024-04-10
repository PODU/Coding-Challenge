package engine

import "testing"

func TestFENRoundTrip(t *testing.T) {
	cases := []string{
		StartFEN,
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		"r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
		"rnbqkbnr/pp1ppppp/8/2p5/4P3/8/PPPP1PPP/RNBQKBNR w KQkq c6 0 2",
		"4k3/8/8/8/8/8/8/4K3 b - - 5 39",
		"4k3/8/8/8/8/8/8/R3K2R w KQ - 0 1",
	}
	for _, fen := range cases {
		b, err := ParseFEN(fen)
		if err != nil {
			t.Errorf("ParseFEN(%q): %v", fen, err)
			continue
		}
		if got := b.FEN(); got != fen {
			t.Errorf("round-trip: got %q, want %q", got, fen)
		}
	}
}

func TestFENShortFormDefaults(t *testing.T) {
	b, err := ParseFEN("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -")
	if err != nil {
		t.Fatal(err)
	}
	if b.HalfmoveClock() != 0 || b.FullmoveNumber() != 1 {
		t.Errorf("defaults: halfmove=%d fullmove=%d, want 0/1", b.HalfmoveClock(), b.FullmoveNumber())
	}
}

func TestFENInvalid(t *testing.T) {
	bad := []string{
		"",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP w KQkq - 0 1",           // 7 ranks
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNX w KQkq - 0 1",  // illegal char X
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBN w KQkq - 0 1",   // rank not summing to 8
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQ1BNR w KQkq - 0 1",  // no white king
		"4k3/8/8/8/8/8/8/4K2K w - - 0 1",                            // two white kings
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1",  // bad side
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w XQkq - 0 1",  // bad castling
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq z9 0 1", // bad ep
	}
	for _, fen := range bad {
		if _, err := ParseFEN(fen); err == nil {
			t.Errorf("ParseFEN(%q) = nil error, want error", fen)
		}
	}
}
