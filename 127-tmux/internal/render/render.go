// Package render draws a session's active window (panes, borders and the
// status bar) into a single grid the size of the client's terminal.
package render

import (
	"fmt"
	"strings"

	"cctmux/internal/layout"
	"cctmux/internal/screen"
	"cctmux/internal/session"
)

const (
	glyphH = '─'
	glyphV = '│'
)

// activeStyle is bold green, used for the borders around the active pane.
const activeStyle = "1;32"

// Render draws the session's active window into a w x h grid. now is the time
// shown in the status bar; it is a parameter so tests get a fixed value.
func Render(s *session.Session, w, h, cols int, now string) *screen.Screen {
	if cols == 0 {
		cols = w
	}
	comp := screen.New(w, h)
	area := layout.Rect{X: 0, Y: 0, W: w, H: h - 1} // last row is the status bar
	win := s.ActiveWindow()
	geo := win.Layout.Geometry(area)
	activeID := win.Layout.Active()

	for id, r := range geo {
		if p := win.Pane(id); p != nil {
			paintPane(comp, p, r)
		}
	}

	activeRect, hasActive := geo[activeID]
	for _, b := range win.Layout.Borders(area) {
		style := ""
		if hasActive && borderTouchesActive(b, activeRect) {
			style = activeStyle
		}
		drawBorder(comp, b, style)
	}

	drawStatus(comp, s, w, h-1, now)

	if hasActive {
		if p := win.Pane(activeID); p != nil && p.Copy {
			paintCopyBanner(comp, activeRect, p)
		}
	}
	return comp
}

func paintPane(comp *screen.Screen, p *session.Pane, r layout.Rect) {
	pw, ph := p.Screen.Size()
	off := 0
	if p.Copy {
		off = p.ScrollOff
	}
	for y := 0; y < r.H && y < ph; y++ {
		for x := 0; x < r.W && x < pw; x++ {
			var c screen.Cell
			if off > 0 {
				c = cellFromHistory(p, x, y, off)
			} else {
				c = p.Screen.Cell(x, y)
			}
			comp.Set(r.X+x, r.Y+y, c.Rune, c.Style)
		}
	}
}

// cellFromHistory returns the cell shown at (x,y) when the pane is scrolled
// back off lines. It treats the scrollback followed by the live screen as one
// long buffer.
func cellFromHistory(p *session.Pane, x, y, off int) screen.Cell {
	sb := p.Screen.Scrollback()
	_, ph := p.Screen.Size()
	vy := len(sb) - off + y
	if vy < 0 {
		return screen.Cell{Rune: ' '}
	}
	if vy < len(sb) {
		line := []rune(sb[vy])
		if x < len(line) {
			return screen.Cell{Rune: line[x]}
		}
		return screen.Cell{Rune: ' '}
	}
	live := vy - len(sb)
	if live < ph {
		return p.Screen.Cell(x, live)
	}
	return screen.Cell{Rune: ' '}
}

func drawBorder(comp *screen.Screen, b layout.Border, style string) {
	if b.Vertical {
		for y := 0; y < b.Rect.H; y++ {
			comp.Set(b.Rect.X, b.Rect.Y+y, glyphV, style)
		}
	} else {
		for x := 0; x < b.Rect.W; x++ {
			comp.Set(b.Rect.X+x, b.Rect.Y, glyphH, style)
		}
	}
}

// borderTouchesActive reports whether b runs along an edge of the active
// pane a.
func borderTouchesActive(b layout.Border, a layout.Rect) bool {
	if b.Vertical {
		return (b.Rect.X == a.X-1 || b.Rect.X == a.X+a.W) &&
			overlap(b.Rect.Y, b.Rect.H, a.Y, a.H)
	}
	return (b.Rect.Y == a.Y-1 || b.Rect.Y == a.Y+a.H) &&
		overlap(b.Rect.X, b.Rect.W, a.X, a.W)
}

func overlap(a0, alen, b0, blen int) bool {
	return a0 < b0+blen && b0 < a0+alen
}

func paintCopyBanner(comp *screen.Screen, r layout.Rect, p *session.Pane) {
	total := p.Screen.ScrollbackLen()
	label := fmt.Sprintf("[COPY %d/%d]", p.ScrollOff, total)
	x := r.X + r.W - len([]rune(label))
	if x < r.X {
		x = r.X
	}
	for i, ch := range label {
		comp.Set(x+i, r.Y, ch, "7") // reverse video
	}
}

// StatusBar returns the status line: session name on the left, windows in the
// middle and the clock on the right.
func StatusBar(s *session.Session, w int, now string) string {
	left := fmt.Sprintf("[%s] ", s.Name)
	right := " " + now
	var center strings.Builder
	for i, win := range s.Windows() {
		marker := ' '
		if i == s.ActiveIndex() {
			marker = '*'
		}
		center.WriteString(fmt.Sprintf("%d:%s%c ", win.ID, win.Name, marker))
	}
	return composeStatus(left, center.String(), right, w)
}

func composeStatus(left, center, right string, w int) string {
	lr := []rune(left)
	cr := []rune(center)
	rr := []rune(right)
	row := []rune(strings.Repeat(" ", w))
	put := func(start int, src []rune) {
		for i, ch := range src {
			p := start + i
			if p >= 0 && p < w {
				row[p] = ch
			}
		}
	}
	put(0, lr)
	put(w-len(rr), rr)
	// Centre the window list in the gap between the two.
	span := max(w-len(lr)-len(rr), 0)
	put(max(len(lr)+(span-len(cr))/2, len(lr)), cr)
	return string(row)
}

func drawStatus(comp *screen.Screen, s *session.Session, w, y int, now string) {
	bar := []rune(StatusBar(s, w, now))
	for x := 0; x < w && x < len(bar); x++ {
		comp.Set(x, y, bar[x], "7") // reverse video
	}
}
