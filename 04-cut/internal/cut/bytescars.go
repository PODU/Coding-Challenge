package cut

import (
	"bufio"
	"unicode/utf8"
)

// cutBytes writes the selected bytes of line. If an output delimiter is set,
// it goes between runs of selected bytes that are not adjacent.
func (c *Cutter) cutBytes(line []byte, w *bufio.Writer) {
	prevSelected := false
	wroteAny := false
	for i := 1; i <= len(line); i++ {
		if c.cfg.Sel.Selected(i) {
			if !prevSelected && wroteAny && c.cfg.OutDelim != "" {
				w.WriteString(c.cfg.OutDelim)
			}
			w.WriteByte(line[i-1])
			wroteAny = true
			prevSelected = true
		} else {
			prevSelected = false
		}
	}
}

// cutChars is cutBytes for UTF-8 characters. Invalid bytes come out as U+FFFD.
func (c *Cutter) cutChars(line []byte, w *bufio.Writer) {
	pos := 1
	prevSelected := false
	wroteAny := false
	rest := line
	for len(rest) > 0 {
		r, size := utf8.DecodeRune(rest)
		if c.cfg.Sel.Selected(pos) {
			if !prevSelected && wroteAny && c.cfg.OutDelim != "" {
				w.WriteString(c.cfg.OutDelim)
			}
			w.WriteRune(r)
			wroteAny = true
			prevSelected = true
		} else {
			prevSelected = false
		}
		rest = rest[size:]
		pos++
	}
}
