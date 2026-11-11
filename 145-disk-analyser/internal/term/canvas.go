package term

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Style is a cell's colours (256-colour palette indexes, -1 for the
// terminal default) and attributes.
type Style struct {
	Fg, Bg    int16
	Bold      bool
	Reverse   bool
	Underline bool
}

var Default = Style{Fg: -1, Bg: -1}

type cell struct {
	r    rune
	s    Style
	cont bool // right half of a double-width character
}

// Canvas is an off-screen grid of cells. The UI draws a whole frame into it
// and the Renderer sends only what changed.
type Canvas struct {
	W, H  int
	cells []cell
}

func NewCanvas(w, h int) *Canvas {
	c := &Canvas{W: w, H: h, cells: make([]cell, w*h)}
	c.Clear()
	return c
}

func (c *Canvas) Clear() {
	for i := range c.cells {
		c.cells[i] = cell{r: ' ', s: Default}
	}
}

func (c *Canvas) Set(x, y int, r rune, s Style) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	c.cells[y*c.W+x] = cell{r: r, s: s}
}

// SetStyle changes a cell's style but keeps its character.
func (c *Canvas) SetStyle(x, y int, s Style) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	c.cells[y*c.W+x].s = s
}

func (c *Canvas) Fill(x, y, w, h int, r rune, s Style) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			c.Set(xx, yy, r, s)
		}
	}
}

// Text writes s from (x, y), clipped to max columns, and returns the
// columns used. Wide characters take two cells and are never split.
func (c *Canvas) Text(x, y int, s string, st Style, max int) int {
	used := 0
	for _, r := range s {
		if r < ' ' || r == 0x7f {
			r = '?' // control characters in file names must not reach the terminal
		}
		w := RuneWidth(r)
		if w == 0 {
			continue
		}
		if used+w > max {
			break
		}
		c.Set(x+used, y, r, st)
		if w == 2 && x+used+1 < c.W && y < c.H && y >= 0 {
			c.cells[y*c.W+x+used+1] = cell{s: st, cont: true}
		}
		used += w
	}
	return used
}

// Row returns a line's characters, for tests.
func (c *Canvas) Row(y int) string {
	var b strings.Builder
	for x := 0; x < c.W; x++ {
		if cl := c.cells[y*c.W+x]; !cl.cont {
			b.WriteRune(cl.r)
		}
	}
	return b.String()
}

// String is the whole canvas as text, for tests.
func (c *Canvas) String() string {
	var b strings.Builder
	for y := 0; y < c.H; y++ {
		b.WriteString(strings.TrimRight(c.Row(y), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// StyleAt returns a cell's style, for tests.
func (c *Canvas) StyleAt(x, y int) Style { return c.cells[y*c.W+x].s }

// TextWidth is the number of columns s occupies.
func TextWidth(s string) int {
	n := 0
	for _, r := range s {
		n += RuneWidth(r)
	}
	return n
}

// Truncate shortens s to max columns, ending with "…" if cut.
func Truncate(s string, max int) string {
	if TextWidth(s) <= max {
		return s
	}
	if max <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := RuneWidth(r)
		if used+w > max-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	b.WriteRune('…')
	return b.String()
}

// RuneWidth is 2 for East Asian wide and emoji code points, 0 for
// combining marks, 1 otherwise. Enough for file names.
func RuneWidth(r rune) int {
	switch {
	case r >= 0x0300 && r <= 0x036F, r >= 0x200B && r <= 0x200F, r == 0xFE0F:
		return 0
	case r >= 0x1100 && r <= 0x115F, r >= 0x2E80 && r <= 0xA4CF, r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF, r >= 0xFE30 && r <= 0xFE4F, r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6, r >= 0x1F300 && r <= 0x1FAFF, r >= 0x20000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}

// Renderer sends the difference between frames as escape codes.
type Renderer struct {
	prev []cell
	w, h int
	buf  []byte
}

func (r *Renderer) Invalidate() { r.prev = nil }

func (r *Renderer) Frame(c *Canvas) []byte {
	r.buf = r.buf[:0]
	if r.prev == nil || r.w != c.W || r.h != c.H {
		r.buf = append(r.buf, "\x1b[0m\x1b[2J"...)
		r.prev = make([]cell, len(c.cells))
		for i := range r.prev {
			r.prev[i] = cell{r: ' ', s: Default}
		}
		r.w, r.h = c.W, c.H
	}
	curX, curY := -1, -1
	var last *Style
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			i := y*c.W + x
			cl := c.cells[i]
			if cl == r.prev[i] || cl.cont {
				r.prev[i] = cl
				continue
			}
			r.prev[i] = cl
			if x != curX || y != curY {
				r.buf = append(r.buf, "\x1b["...)
				r.buf = strconv.AppendInt(r.buf, int64(y+1), 10)
				r.buf = append(r.buf, ';')
				r.buf = strconv.AppendInt(r.buf, int64(x+1), 10)
				r.buf = append(r.buf, 'H')
			}
			if last == nil || *last != cl.s {
				r.buf = appendSGR(r.buf, cl.s)
				s := cl.s
				last = &s
			}
			r.buf = utf8.AppendRune(r.buf, cl.r)
			curX, curY = x+RuneWidth(cl.r), y
			if RuneWidth(cl.r) == 2 && x+1 < c.W {
				r.prev[i+1] = c.cells[i+1]
			}
		}
	}
	return r.buf
}

func appendSGR(b []byte, s Style) []byte {
	b = append(b, "\x1b[0"...)
	if s.Bold {
		b = append(b, ";1"...)
	}
	if s.Underline {
		b = append(b, ";4"...)
	}
	if s.Reverse {
		b = append(b, ";7"...)
	}
	if s.Fg >= 0 {
		b = append(b, ";38;5;"...)
		b = strconv.AppendInt(b, int64(s.Fg), 10)
	}
	if s.Bg >= 0 {
		b = append(b, ";48;5;"...)
		b = strconv.AppendInt(b, int64(s.Bg), 10)
	}
	return append(b, 'm')
}

// Setup and teardown sequences.
const (
	EnterUI = "\x1b[?1049h\x1b[?25l\x1b[?7l\x1b[?1000h\x1b[?1006h\x1b[0m\x1b[2J"
	LeaveUI = "\x1b[0m\x1b[?1006l\x1b[?1000l\x1b[?7h\x1b[?25h\x1b[?1049l"
)
