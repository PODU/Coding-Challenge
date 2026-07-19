package render

import (
	"strings"

	"cctmux/internal/screen"
)

// Emit turns grid into the bytes that draw it on a terminal, starting from the
// top-left corner. SGR codes are only written where the style changes.
func Emit(grid *screen.Screen) []byte {
	w, h := grid.Size()
	var b strings.Builder
	b.WriteString("\x1b[H")
	curStyle := "\x00" // matches no real style, so the first cell sets one
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := grid.Cell(x, y)
			if c.Style != curStyle {
				if c.Style == "" {
					b.WriteString("\x1b[0m")
				} else {
					b.WriteString("\x1b[" + c.Style + "m")
				}
				curStyle = c.Style
			}
			r := c.Rune
			if r == 0 {
				r = ' '
			}
			b.WriteRune(r)
		}
		if y < h-1 {
			b.WriteString("\x1b[0m\r\n")
			curStyle = ""
		}
	}
	b.WriteString("\x1b[0m")
	return []byte(b.String())
}
