package engine

import "testing"

// Reference perft node counts from well-known positions (Chess Programming
// Wiki). These exercise castling, en passant, promotion, and self-check
// legality in combination and are the primary move-generation correctness gate.
type perftCase struct {
	name  string
	fen   string
	depth int
	nodes uint64
	long  bool // gated behind -short=false (deep / slow)
}

var perftCases = []perftCase{
	// Starting position.
	{"startpos-1", StartFEN, 1, 20, false},
	{"startpos-2", StartFEN, 2, 400, false},
	{"startpos-3", StartFEN, 3, 8902, false},
	{"startpos-4", StartFEN, 4, 197281, false},
	{"startpos-5", StartFEN, 5, 4865609, true},

	// Kiwipete.
	{"kiwipete-1", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -", 1, 48, false},
	{"kiwipete-2", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -", 2, 2039, false},
	{"kiwipete-3", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -", 3, 97862, false},
	{"kiwipete-4", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq -", 4, 4085603, true},

	// Position 3: en passant and pins.
	{"pos3-1", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - -", 1, 14, false},
	{"pos3-2", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - -", 2, 191, false},
	{"pos3-3", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - -", 3, 2812, false},
	{"pos3-4", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - -", 4, 43238, false},
	{"pos3-5", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - -", 5, 674624, true},

	// Position 4 (and its mirror is symmetric).
	{"pos4-1", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq -", 1, 6, false},
	{"pos4-2", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq -", 2, 264, false},
	{"pos4-3", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq -", 3, 9467, false},

	// Position 5 and 6 (additional standard references).
	{"pos5-1", "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ -", 1, 44, false},
	{"pos5-3", "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ -", 3, 62379, false},
	{"pos6-3", "r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - -", 3, 89890, false},
}

func TestPerft(t *testing.T) {
	short := testing.Short()
	for _, tc := range perftCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.long && short {
				t.Skip("deep perft skipped in -short mode")
			}
			b, err := ParseFEN(tc.fen)
			if err != nil {
				t.Fatalf("ParseFEN(%q): %v", tc.fen, err)
			}
			got := b.Perft(tc.depth)
			if got != tc.nodes {
				t.Errorf("Perft(%d) = %d, want %d", tc.depth, got, tc.nodes)
			}
		})
	}
}
