package main

import (
	"fmt"
	"io"
	"strings"
)

// hexSink writes a `hexdump -C` style dump of everything written to it, with
// the offset carrying on from one write to the next.
type hexSink struct {
	w   io.Writer
	off int
}

func newHexSink(w io.Writer) *hexSink {
	return &hexSink{w: w}
}

func (h *hexSink) Write(p []byte) (int, error) {
	hexDump(h.w, h.off, p)
	h.off += len(p)
	return len(p), nil
}

// hexDump writes data in `hexdump -C` format, numbering from offset base:
//
//	00000000  68 69 0a                                          |hi.|
func hexDump(w io.Writer, base int, data []byte) {
	for i := 0; i < len(data); i += 16 {
		row := data[i:min(i+16, len(data))]

		var b strings.Builder
		fmt.Fprintf(&b, "%08x  ", base+i)
		for j := 0; j < 16; j++ {
			if j == 8 {
				b.WriteByte(' ')
			}
			if j < len(row) {
				fmt.Fprintf(&b, "%02x ", row[j])
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" |")
		for _, c := range row {
			if c >= 0x20 && c <= 0x7e {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|\n")
		io.WriteString(w, b.String())
	}
}
