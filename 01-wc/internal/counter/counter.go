// Package counter computes all of wc's counts in a single pass over the input.
package counter

import (
	"bufio"
	"io"
	"unicode/utf8"
)

// Counts holds the results for one input.
type Counts struct {
	Bytes int64
	Lines int64 // '\n' bytes
	Words int64
	Chars int64 // UTF-8 runes
}

// Add adds o into c, for the "total" row.
func (c *Counts) Add(o Counts) {
	c.Bytes += o.Bytes
	c.Lines += o.Lines
	c.Words += o.Words
	c.Chars += o.Chars
}

// isSpace matches C's isspace() in the "C" locale, which is what wc splits
// words on.
func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}

const readBufSize = 64 * 1024

// Count reads r to EOF. As in GNU wc, a last line without a newline is not
// counted as a line, and each invalid UTF-8 byte counts as one character. On a
// read error the counts so far are returned with the error.
func Count(r io.Reader) (Counts, error) {
	var c Counts

	br := bufio.NewReaderSize(r, readBufSize)
	buf := make([]byte, readBufSize)

	inWord := false
	// carry holds a rune split across two reads. It must not alias buf, which
	// the next Read overwrites.
	carry := make([]byte, 0, utf8.UTFMax)
	var combined []byte

	for {
		n, err := br.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			c.Bytes += int64(n)

			// All the delimiters are ASCII, so lines and words can be counted
			// on raw bytes.
			for _, b := range chunk {
				if b == '\n' {
					c.Lines++
				}
				if isSpace(b) {
					inWord = false
				} else if !inWord {
					inWord = true
					c.Words++
				}
			}

			data := chunk
			if len(carry) > 0 {
				combined = append(combined[:0], carry...)
				combined = append(combined, chunk...)
				data = combined
				carry = carry[:0]
			}
			// FullRune is also true for a single invalid byte, so this only
			// stops on a truncated rune at the end of the chunk.
			for len(data) > 0 && utf8.FullRune(data) {
				_, size := utf8.DecodeRune(data)
				c.Chars++
				data = data[size:]
			}
			carry = append(carry[:0], data...)
		}

		if err != nil {
			if err == io.EOF {
				// A truncated rune at EOF counts one char per byte.
				c.Chars += int64(len(carry))
				return c, nil
			}
			return c, err
		}
	}
}
