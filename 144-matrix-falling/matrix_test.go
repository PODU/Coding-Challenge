package main

import (
	"math/rand/v2"
	"testing"
	"unicode/utf8"
)

func newTestMatrix(w, h int, o Options) *Matrix {
	return NewMatrix(w, h, newSimConfig(o), rand.New(rand.NewPCG(1, 2)))
}

func defaultOpts() Options { return Options{Delay: 4, Colour: colGreen} }

// checkColumns verifies the trail invariants: at most one head per column,
// the head is the lowest lit cell, and the lit cells are contiguous above it.
func checkColumns(t *testing.T, m *Matrix) {
	t.Helper()
	for x := 0; x < m.W; x++ {
		c := m.cols[x]
		for y := 0; y < m.H; y++ {
			cell := m.At(x, y)
			lit := c.active && y <= c.head && y > c.head-c.length
			if lit != (cell.Kind != Blank) {
				t.Fatalf("col %d row %d: kind %d, head %d len %d active %v", x, y, cell.Kind, c.head, c.length, c.active)
			}
			if cell.Kind == Head && y != c.head {
				t.Fatalf("col %d: head drawn at row %d but column head is %d", x, y, c.head)
			}
		}
	}
}

func TestSingleTrailFalls(t *testing.T) {
	m := newTestMatrix(1, 20, defaultOpts())
	m.cols[0] = column{} // start immediately
	m.Step()
	if m.At(0, 0).Kind != Head {
		t.Fatalf("first step should draw a head at the top, got %+v", m.At(0, 0))
	}
	every, length := m.cols[0].every, m.cols[0].length
	for i := 0; i < every; i++ {
		m.Step()
	}
	if m.At(0, 1).Kind != Head || m.At(0, 0).Kind != Bright {
		t.Fatalf("head should move down one row and leave a bright trail: %+v %+v", m.At(0, 0), m.At(0, 1))
	}
	for i := 0; i < every*(length+2); i++ {
		m.Step()
		checkColumns(t, m)
	}
	if m.At(0, 0).Kind != Blank {
		t.Fatal("rows above the trail should be blank again")
	}
}

func TestTrailFades(t *testing.T) {
	if shade(1, 12) != Bright || shade(5, 12) != Normal || shade(11, 12) != Dim {
		t.Fatal("trail should fade bright -> normal -> dim")
	}
}

func TestManyColumnsVaryAndStayConsistent(t *testing.T) {
	m := newTestMatrix(80, 24, defaultOpts())
	speeds := map[int]bool{}
	for i := 0; i < 500; i++ {
		m.Step()
		checkColumns(t, m)
		for _, c := range m.cols {
			if c.active {
				speeds[c.every] = true
			}
		}
	}
	if len(speeds) < 2 {
		t.Fatalf("columns should fall at different speeds, saw %v", speeds)
	}
	lit := 0
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			if m.At(x, y).Kind != Blank {
				lit++
			}
		}
	}
	if lit < m.W*m.H/10 {
		t.Fatalf("screen should be well filled after warm-up, only %d cells lit", lit)
	}
}

func TestColumnsStartAtDifferentTimes(t *testing.T) {
	m := newTestMatrix(80, 24, defaultOpts())
	waits := map[int]bool{}
	for _, c := range m.cols {
		waits[c.wait] = true
	}
	if len(waits) < 10 {
		t.Fatalf("start times should be random, got %d distinct", len(waits))
	}
}

func TestTrailColourAndRainbow(t *testing.T) {
	o := defaultOpts()
	o.Colour = colRed
	m := newTestMatrix(40, 20, o)
	for i := 0; i < 200; i++ {
		m.Step()
	}
	for _, c := range m.cells {
		if c.Kind != Blank && c.Color != colRed {
			t.Fatalf("cell colour %d, want red", c.Color)
		}
	}

	o.Rainbow = true
	m = newTestMatrix(40, 20, o)
	for i := 0; i < 200; i++ {
		m.Step()
	}
	colours := map[uint8]bool{}
	for _, c := range m.cells {
		if c.Kind != Blank {
			colours[c.Color] = true
		}
	}
	if len(colours) < 4 {
		t.Fatalf("rainbow should use several colours, got %v", colours)
	}
}

func TestKatakana(t *testing.T) {
	o := defaultOpts()
	o.Katakana = true
	m := newTestMatrix(40, 20, o)
	for i := 0; i < 200; i++ {
		m.Step()
	}
	seen := false
	for _, c := range m.cells {
		if c.Kind == Blank {
			continue
		}
		seen = true
		if c.Ch < 0xFF66 || c.Ch > 0xFF9D {
			t.Fatalf("%U is not half-width katakana", c.Ch)
		}
	}
	if !seen {
		t.Fatal("nothing drawn")
	}
	if len(katakana) != 0xFF9D-0xFF66+1 || !utf8.ValidRune(katakana[0]) {
		t.Fatal("bad katakana table")
	}
}

func TestBold(t *testing.T) {
	o := defaultOpts()
	o.Bold = boldAll
	m := newTestMatrix(40, 20, o)
	for i := 0; i < 200; i++ {
		m.Step()
	}
	for _, c := range m.cells {
		if c.Kind != Blank && !c.Bold {
			t.Fatal("-B should make every character bold")
		}
	}
}

func TestResizeSmallerAndLarger(t *testing.T) {
	m := newTestMatrix(80, 24, defaultOpts())
	for i := 0; i < 100; i++ {
		m.Step()
	}
	sizes := [][2]int{{30, 10}, {120, 40}, {1, 1}, {0, 0}, {80, 24}, {200, 60}, {10, 50}}
	for _, sz := range sizes {
		m.Resize(sz[0], sz[1])
		if len(m.cells) != sz[0]*sz[1] || len(m.cols) != sz[0] {
			t.Fatalf("resize %v: grid %d cols %d", sz, len(m.cells), len(m.cols))
		}
		for i := 0; i < 150; i++ {
			m.Step() // must not index outside the grid
		}
	}
	// Once every column has cycled, the invariants hold again everywhere.
	for i := 0; i < 2000; i++ {
		m.Step()
	}
	checkColumns(t, m)
}

func TestResizeKeepsOverlap(t *testing.T) {
	m := newTestMatrix(10, 10, defaultOpts())
	for i := 0; i < 60; i++ {
		m.Step()
	}
	before := m.At(3, 4)
	m.Resize(5, 6)
	if m.At(3, 4) != before {
		t.Fatal("overlapping cells should survive a resize")
	}
}

func TestFlickerChangesCharacters(t *testing.T) {
	o := defaultOpts()
	o.Flicker = true
	m := newTestMatrix(1, 60, o)
	m.cols[0] = column{}
	m.Step()
	m.cols[0].length, m.cols[0].every = 40, 1
	for i := 0; i < 20; i++ {
		m.Step()
	}
	snap := append([]Cell(nil), m.cells...)
	changed := false
	for i := 0; i < 10 && !changed; i++ {
		m.Step()
		for y := 0; y < 20; y++ {
			if m.cells[y].Kind != Blank && snap[y].Kind != Blank && m.cells[y].Ch != snap[y].Ch {
				changed = true
			}
		}
	}
	if !changed {
		t.Fatal("-f should change trail characters as they fall")
	}
}
