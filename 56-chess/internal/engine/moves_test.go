package engine

import (
	"sort"
	"strings"
	"testing"
)

// legalSet returns the set of legal moves as coordinate strings.
func legalSet(t *testing.T, fen string) map[string]bool {
	t.Helper()
	b, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN(%q): %v", fen, err)
	}
	set := map[string]bool{}
	for _, m := range b.LegalMoves() {
		set[m.String()] = true
	}
	return set
}

func movesFrom(t *testing.T, fen, from string) []string {
	t.Helper()
	var out []string
	for m := range legalSet(t, fen) {
		if strings.HasPrefix(m, from) {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

func TestCastlingAllowed(t *testing.T) {
	set := legalSet(t, "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1")
	if !set["e1g1"] {
		t.Error("white king-side castling should be legal")
	}
	if !set["e1c1"] {
		t.Error("white queen-side castling should be legal")
	}
}

func TestCastlingBlockedByOccupancy(t *testing.T) {
	// Knight on b1 blocks queen-side; bishop on f1 blocks king-side.
	set := legalSet(t, "r3k2r/8/8/8/8/8/8/RN2KB1R w KQkq - 0 1")
	if set["e1c1"] {
		t.Error("queen-side castling blocked by b1 occupancy")
	}
	if set["e1g1"] {
		t.Error("king-side castling blocked by f1 occupancy")
	}
}

func TestCastlingThroughCheck(t *testing.T) {
	// Black rook on f8 attacks f1: king may not pass through f1.
	set := legalSet(t, "r4rk1/8/8/8/8/8/8/R3K2R w KQ - 0 1")
	if set["e1g1"] {
		t.Error("king-side castling through attacked f1 must be illegal")
	}
}

func TestCastlingOutOfCheck(t *testing.T) {
	// King in check by rook on e8: cannot castle.
	set := legalSet(t, "4r2k/8/8/8/8/8/8/R3K2R w KQ - 0 1")
	if set["e1g1"] || set["e1c1"] {
		t.Error("cannot castle while in check")
	}
}

func TestCastlingRightsLostByRookCapture(t *testing.T) {
	// White knight on g6 can capture black rook on h8, removing black's
	// king-side castling right.
	b, err := ParseFEN("r3k2r/8/6N1/8/8/8/8/R3K2R w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := ParseMove(b, "g6h8")
	if !ok {
		t.Fatal("g6h8 should be legal")
	}
	b.MakeMove(m)
	if b.CastlingRights()&CastleBK != 0 {
		t.Error("capturing the h8 rook must clear black king-side castling right")
	}
}

func TestEnPassantAvailableOnePly(t *testing.T) {
	// After ...c7c5, white pawn on b5 may capture en passant on c6.
	b, err := ParseFEN("rnbqkbnr/pp1ppppp/8/1Pp5/8/8/P1PPPPPP/RNBQKBNR w KQkq c6 0 2")
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, m := range b.LegalMoves() {
		set[m.String()] = true
	}
	if !set["b5c6"] {
		t.Error("en passant b5c6 should be available")
	}
	// Verify the captured pawn is removed.
	m, _ := ParseMove(b, "b5c6")
	b.MakeMove(m)
	if b.PieceAt(Sq(2, 4)) != Empty { // c5
		t.Error("en passant should remove the passed pawn on c5")
	}
}

func TestEnPassantPinned(t *testing.T) {
	// Classic horizontal-pin position: white king a5, black rook h5, white pawn
	// b5, black pawn c5 (ep target c6). Taking en passant would expose the king
	// to the rook along the 5th rank, so it is illegal.
	set := legalSet(t, "7k/8/8/K1Pp3r/8/8/8/8 w - d6 0 1")
	if set["c5d6"] {
		t.Error("en passant exposing own king along the rank must be illegal")
	}
}

func TestPromotionFourOptions(t *testing.T) {
	moves := movesFrom(t, "8/P7/8/8/8/8/8/k6K w - - 0 1", "a7a8")
	want := []string{"a7a8b", "a7a8n", "a7a8q", "a7a8r"}
	if strings.Join(moves, ",") != strings.Join(want, ",") {
		t.Errorf("promotion options = %v, want %v", moves, want)
	}
}

func TestPromotionWithCapture(t *testing.T) {
	// Pawn on b7 can promote by capturing the rook on a8 or c8, or push to b8.
	set := legalSet(t, "r1r4k/1P6/8/8/8/8/8/7K w - - 0 1")
	for _, mv := range []string{"b7a8q", "b7c8q", "b7b8q"} {
		if !set[mv] {
			t.Errorf("expected promotion move %s to be legal", mv)
		}
	}
}

func TestPinnedPieceCannotMoveOffRay(t *testing.T) {
	// White knight on e2 is pinned to the king on e1 by the rook on e8.
	set := legalSet(t, "4r2k/8/8/8/8/8/4N3/4K3 w - - 0 1")
	for mv := range set {
		if strings.HasPrefix(mv, "e2") {
			t.Errorf("pinned knight should have no moves, got %s", mv)
		}
	}
}
