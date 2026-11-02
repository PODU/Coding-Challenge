package main

import (
	"strings"
	"testing"
)

func TestFirstFrameClearsAndDraws(t *testing.T) {
	m := newTestMatrix(3, 3, defaultOpts())
	m.cells[0] = Cell{Ch: 'A', Kind: Head, Color: colGreen}
	m.cells[4] = Cell{Ch: 'B', Kind: Bright, Color: colGreen}
	var r Renderer
	out := string(r.Frame(m))
	if !strings.HasPrefix(out, escReset+escClear) {
		t.Fatalf("first frame should clear: %q", out)
	}
	if !strings.Contains(out, "\x1b[1;1H\x1b[0;1;97mA") {
		t.Fatalf("head should be bright white at 1,1: %q", out)
	}
	if !strings.Contains(out, "\x1b[2;2H\x1b[0;1;32mB") {
		t.Fatalf("bright trail should be bold green at 2,2: %q", out)
	}
}

func TestOnlyChangesAreRedrawn(t *testing.T) {
	m := newTestMatrix(3, 3, defaultOpts())
	var r Renderer
	r.Frame(m)
	if out := r.Frame(m); len(out) != 0 {
		t.Fatalf("unchanged frame should produce no output, got %q", out)
	}
	m.cells[8] = Cell{Ch: 'Z', Kind: Dim, Color: colBlue}
	out := string(r.Frame(m))
	if out != "\x1b[3;3H\x1b[0;2;34mZ" {
		t.Fatalf("got %q", out)
	}
	m.cells[8] = Cell{}
	if out := string(r.Frame(m)); out != "\x1b[3;3H"+escReset+" " {
		t.Fatalf("blanking should write a reset space, got %q", out)
	}
}

func TestAdjacentCellsSkipCursorMove(t *testing.T) {
	m := newTestMatrix(3, 1, defaultOpts())
	var r Renderer
	r.Frame(m)
	m.cells[0] = Cell{Ch: 'a', Kind: Normal, Color: colGreen}
	m.cells[1] = Cell{Ch: 'b', Kind: Normal, Color: colGreen}
	out := string(r.Frame(m))
	if out != "\x1b[1;1H\x1b[0;32mab" {
		t.Fatalf("got %q", out)
	}
}

func TestResizeTriggersFullRedraw(t *testing.T) {
	m := newTestMatrix(3, 3, defaultOpts())
	var r Renderer
	r.Frame(m)
	m.Resize(4, 2)
	if out := string(r.Frame(m)); !strings.HasPrefix(out, escReset+escClear) {
		t.Fatalf("resized frame should clear first: %q", out)
	}
	r.Invalidate()
	if out := string(r.Frame(m)); !strings.HasPrefix(out, escReset+escClear) {
		t.Fatalf("invalidated frame should clear first: %q", out)
	}
}

func TestKatakanaIsUTF8(t *testing.T) {
	m := newTestMatrix(1, 1, defaultOpts())
	m.cells[0] = Cell{Ch: 0xFF71, Kind: Normal, Color: colGreen}
	var r Renderer
	if out := string(r.Frame(m)); !strings.HasSuffix(out, "ｱ") {
		t.Fatalf("got %q", out)
	}
}

func TestSGRBoldOverridesDim(t *testing.T) {
	if s := sgr(Cell{Kind: Dim, Color: colRed, Bold: true}); s != "\x1b[0;1;31m" {
		t.Fatalf("got %q", s)
	}
}
