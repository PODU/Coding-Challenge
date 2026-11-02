package main

import (
	"strconv"
	"unicode/utf8"
)

// Terminal control sequences. Everything is plain ANSI/VT escape codes.
const (
	escAltScreenOn  = "\x1b[?1049h" // save the screen and switch to a blank one
	escAltScreenOff = "\x1b[?1049l" // ...and bring the original back
	escHideCursor   = "\x1b[?25l"
	escShowCursor   = "\x1b[?25h"
	escWrapOff      = "\x1b[?7l" // writing the bottom-right cell must not scroll
	escWrapOn       = "\x1b[?7h"
	escMouseOn      = "\x1b[?1000h\x1b[?1006h" // report clicks, SGR encoding
	escMouseOff     = "\x1b[?1006l\x1b[?1000l"
	escReset        = "\x1b[0m"
	escClear        = "\x1b[2J"
)

// Renderer turns a Matrix into the escape codes needed to update the screen.
// It remembers the previous frame and only redraws cells that changed, which
// keeps output (and CPU use) small at high speeds.
type Renderer struct {
	prev []Cell
	w, h int
	buf  []byte
}

// Invalidate forces the next frame to clear and repaint the whole screen.
func (r *Renderer) Invalidate() { r.prev = nil }

func (r *Renderer) Frame(m *Matrix) []byte {
	r.buf = r.buf[:0]
	if r.prev == nil || r.w != m.W || r.h != m.H {
		r.buf = append(r.buf, escReset+escClear...)
		r.prev = make([]Cell, m.W*m.H)
		r.w, r.h = m.W, m.H
	}
	curX, curY := -1, -1
	lastSGR := ""
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			c := m.At(x, y)
			i := y*m.W + x
			if c == r.prev[i] {
				continue
			}
			r.prev[i] = c
			if x != curX || y != curY {
				r.buf = append(r.buf, "\x1b["...)
				r.buf = strconv.AppendInt(r.buf, int64(y+1), 10)
				r.buf = append(r.buf, ';')
				r.buf = strconv.AppendInt(r.buf, int64(x+1), 10)
				r.buf = append(r.buf, 'H')
			}
			if s := sgr(c); s != lastSGR {
				r.buf = append(r.buf, s...)
				lastSGR = s
			}
			if c.Kind == Blank {
				r.buf = append(r.buf, ' ')
			} else {
				r.buf = utf8.AppendRune(r.buf, c.Ch)
			}
			curX, curY = x+1, y
		}
	}
	return r.buf
}

var sgrCache = map[Cell]string{}

// sgr is the Select Graphic Rendition sequence for a cell. Bold usually also
// selects the bright variant of the colour, and faint (2) the darker one,
// which gives the fade with only the eight base colours.
func sgr(c Cell) string {
	key := Cell{Kind: c.Kind, Color: c.Color, Bold: c.Bold}
	if s, ok := sgrCache[key]; ok {
		return s
	}
	var s string
	switch c.Kind {
	case Blank:
		s = escReset
	case Head:
		s = "\x1b[0;1;97m"
	default:
		attr := ""
		switch {
		case c.Kind == Bright || c.Bold:
			attr = "1;"
		case c.Kind == Dim:
			attr = "2;"
		}
		s = "\x1b[0;" + attr + "3" + strconv.Itoa(int(c.Color)) + "m"
	}
	sgrCache[key] = s
	return s
}
