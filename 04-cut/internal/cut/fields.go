package cut

import (
	"bufio"
	"bytes"
)

// cutFields writes the selected fields of line and reports whether the line
// should be printed at all. Lines without a delimiter are passed through as-is,
// or dropped entirely under -s.
func (c *Cutter) cutFields(line []byte, w *bufio.Writer) bool {
	if bytes.IndexByte(line, c.cfg.Delim) < 0 {
		if c.cfg.OnlyDelim {
			return false
		}
		w.Write(line)
		return true
	}

	pos := 1
	wroteAny := false
	rest := line
	for {
		idx := bytes.IndexByte(rest, c.cfg.Delim)
		var field []byte
		last := false
		if idx < 0 {
			field = rest
			last = true
		} else {
			field = rest[:idx]
			rest = rest[idx+1:]
		}

		if c.cfg.Sel.Selected(pos) {
			if wroteAny {
				w.WriteString(c.cfg.OutDelim)
			}
			w.Write(field)
			wroteAny = true
		}

		if last {
			break
		}
		pos++
	}
	return true
}
