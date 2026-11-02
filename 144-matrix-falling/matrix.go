package main

import "math/rand/v2"

// Kind is how a cell is drawn. The trail fades from Bright near the head to
// Dim at its end.
type Kind uint8

const (
	Blank Kind = iota
	Head
	Bright
	Normal
	Dim
)

type Cell struct {
	Ch    rune
	Kind  Kind
	Color uint8 // trail colour; heads are always drawn white
	Bold  bool
}

// column is one falling trail. Rows head-length+1 .. head are lit; the head
// can be above the screen (negative) or below it while the tail drains.
type column struct {
	active bool
	wait   int // frames to wait before the next trail starts
	head   int
	length int
	every  int // frames per one-row move; this is the column's speed
	count  int
}

type simConfig struct {
	chars   []rune
	colour  uint8
	rainbow bool
	bold    int
	flicker bool
}

// Matrix is the screen model. It knows nothing about terminals: Step advances
// the animation by one frame and At reports what each cell should show.
type Matrix struct {
	W, H  int
	cells []Cell
	cols  []column
	rng   *rand.Rand
	cfg   simConfig
}

var alnum = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

// katakana is the half-width block U+FF66..U+FF9D. Each one is a single cell
// wide, so the grid arithmetic is the same as for ASCII.
var katakana = func() []rune {
	var r []rune
	for c := rune(0xFF66); c <= 0xFF9D; c++ {
		r = append(r, c)
	}
	return r
}()

func newSimConfig(o Options) simConfig {
	cfg := simConfig{chars: alnum, colour: o.Colour, rainbow: o.Rainbow, bold: o.Bold, flicker: o.Flicker}
	if o.Katakana {
		cfg.chars = katakana
	}
	return cfg
}

func NewMatrix(w, h int, cfg simConfig, rng *rand.Rand) *Matrix {
	m := &Matrix{rng: rng, cfg: cfg}
	m.Resize(w, h)
	return m
}

func (m *Matrix) At(x, y int) Cell { return m.cells[y*m.W+x] }

// Resize keeps whatever overlaps the old and new screens and starts fresh
// columns for any new width. Columns whose trails now hang below the bottom
// drain naturally on the next steps.
func (m *Matrix) Resize(w, h int) {
	w, h = max(w, 0), max(h, 0)
	cells := make([]Cell, w*h)
	for y := 0; y < min(h, m.H); y++ {
		copy(cells[y*w:y*w+min(w, m.W)], m.cells[y*m.W:y*m.W+min(w, m.W)])
	}
	cols := make([]column, w)
	copy(cols, m.cols)
	oldW := min(len(m.cols), w)
	m.W, m.H, m.cells, m.cols = w, h, cells, cols
	for x := oldW; x < w; x++ {
		m.cols[x] = column{wait: m.rng.IntN(2*h + 1)}
	}
}

func (m *Matrix) Step() {
	for x := range m.cols {
		m.stepColumn(x)
	}
}

func (m *Matrix) stepColumn(x int) {
	c := &m.cols[x]
	if !c.active {
		if c.wait > 0 {
			c.wait--
			return
		}
		m.start(c)
	}
	c.count++
	if c.count < c.every {
		return
	}
	c.count = 0
	c.head++
	if c.head >= 0 && c.head < m.H {
		m.cells[c.head*m.W+x] = m.newCell()
	}
	tail := c.head - c.length // the row that just left the trail
	if tail >= 0 && tail < m.H {
		m.cells[tail*m.W+x] = Cell{}
	}
	for d := 1; d < c.length; d++ {
		y := c.head - d
		if y < 0 {
			break
		}
		if y < m.H {
			m.cells[y*m.W+x].Kind = shade(d, c.length)
		}
	}
	if m.cfg.flicker && m.rng.IntN(3) == 0 {
		y := c.head - 1 - m.rng.IntN(c.length-1)
		if y >= 0 && y < m.H && m.cells[y*m.W+x].Kind != Blank {
			m.cells[y*m.W+x].Ch = m.randChar()
		}
	}
	if tail >= m.H-1 {
		c.active = false
		c.wait = m.rng.IntN(m.H/2 + 1)
	}
}

// start begins a new trail with a random length and speed, so neighbouring
// columns never fall in step.
func (m *Matrix) start(c *column) {
	lo := 4
	hi := max(lo+1, m.H*3/4)
	*c = column{
		active: true,
		head:   -1,
		length: lo + m.rng.IntN(hi-lo),
		every:  1 + m.rng.IntN(3),
	}
	c.count = c.every - 1 // the first move happens on this frame
}

// shade fades the trail in thirds: bright near the head, dim at the end.
func shade(d, length int) Kind {
	switch d * 3 / length {
	case 0:
		return Bright
	case 1:
		return Normal
	default:
		return Dim
	}
}

func (m *Matrix) newCell() Cell {
	c := Cell{Ch: m.randChar(), Kind: Head, Color: m.cfg.colour}
	if m.cfg.rainbow {
		c.Color = rainbowColours[m.rng.IntN(len(rainbowColours))]
	}
	switch m.cfg.bold {
	case boldAll:
		c.Bold = true
	case boldSome:
		c.Bold = m.rng.IntN(2) == 0
	}
	return c
}

func (m *Matrix) randChar() rune { return m.cfg.chars[m.rng.IntN(len(m.cfg.chars))] }
