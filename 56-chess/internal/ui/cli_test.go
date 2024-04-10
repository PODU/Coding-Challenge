package ui

import (
	"bytes"
	"strings"
	"testing"

	"chess/internal/engine"
)

func newTestCLI() (*CLI, *bytes.Buffer) {
	out := &bytes.Buffer{}
	c := NewCLI(strings.NewReader(""), out, Config{Mode: HumanVsHuman, Depth: 0, Seed: 1})
	return c, out
}

func TestRenderStartPosition(t *testing.T) {
	b := engine.NewBoard()
	got := Render(b, false)
	// White back rank present, files labelled.
	if !strings.Contains(got, "R N B Q K B N R") {
		t.Errorf("render missing white back rank:\n%s", got)
	}
	if !strings.Contains(got, "a b c d e f g h") {
		t.Errorf("render missing file labels:\n%s", got)
	}
	if !strings.Contains(got, "r n b q k b n r") {
		t.Errorf("render missing black back rank:\n%s", got)
	}
}

func TestParseAcceptsCoordinateMove(t *testing.T) {
	c, _ := newTestCLI()
	before := c.Board().FEN()
	c.ProcessLine("e2e4")
	if c.Board().FEN() == before {
		t.Error("e2e4 should have updated the position")
	}
	if c.Board().PieceAt(engine.Sq(4, 3)) != engine.WPawn {
		t.Error("e4 should hold a white pawn after e2e4")
	}
}

func TestParseAcceptsPromotion(t *testing.T) {
	c, _ := newTestCLI()
	c.ProcessLine("fen 8/P7/8/8/8/8/8/k6K w - - 0 1")
	c.ProcessLine("a7a8q")
	if c.Board().PieceAt(engine.Sq(0, 7)) != engine.WQueen {
		t.Error("a7a8q should promote to a white queen on a8")
	}
}

func TestRejectsIllegalMoveWithoutMutating(t *testing.T) {
	c, out := newTestCLI()
	before := c.Board().FEN()
	c.ProcessLine("e2e5") // illegal pawn jump
	if c.Board().FEN() != before {
		t.Error("illegal move must not mutate the position")
	}
	if !strings.Contains(out.String(), "Illegal") {
		t.Errorf("expected an illegal-move message, got %q", out.String())
	}
}

func TestRejectsMalformedInput(t *testing.T) {
	c, _ := newTestCLI()
	before := c.Board().FEN()
	for _, bad := range []string{"hello", "z9z9", "e2", "e2e9"} {
		c.ProcessLine(bad)
		if c.Board().FEN() != before {
			t.Errorf("malformed input %q must not mutate the position", bad)
		}
	}
}

func TestUndo(t *testing.T) {
	c, _ := newTestCLI()
	start := c.Board().FEN()
	c.ProcessLine("e2e4")
	c.ProcessLine("undo")
	if c.Board().FEN() != start {
		t.Errorf("undo should restore the start position, got %s", c.Board().FEN())
	}
}

func TestQuitCommand(t *testing.T) {
	c, _ := newTestCLI()
	if !c.ProcessLine("quit") {
		t.Error("quit should signal exit")
	}
}

func TestInvalidFENRejected(t *testing.T) {
	c, out := newTestCLI()
	before := c.Board().FEN()
	c.ProcessLine("fen not-a-fen")
	if c.Board().FEN() != before {
		t.Error("invalid FEN must not replace the board")
	}
	if !strings.Contains(out.String(), "Invalid FEN") {
		t.Errorf("expected an invalid-FEN message, got %q", out.String())
	}
}
