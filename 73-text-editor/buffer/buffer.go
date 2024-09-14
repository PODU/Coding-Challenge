// Package buffer holds the document being edited, as a list of rows.
package buffer

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// TabStop is the tab width in columns.
const TabStop = 8

// Row is one line of the document. render is chars with tabs expanded, and
// must be refreshed whenever chars changes.
type Row struct {
	chars  []byte
	render string
}

// Chars returns the row's bytes. The slice is not a copy; don't modify it.
func (r *Row) Chars() []byte { return r.chars }

// Len returns the length of the row in bytes.
func (r *Row) Len() int { return len(r.chars) }

// String returns the row's text.
func (r *Row) String() string { return string(r.chars) }

// Render returns the row as displayed, with tabs expanded.
func (r *Row) Render() string { return r.render }

// RenderLen returns the displayed width of the row.
func (r *Row) RenderLen() int { return len(r.render) }

func (r *Row) updateRender() {
	var b strings.Builder
	col := 0
	for _, c := range r.chars {
		if c == '\t' {
			b.WriteByte(' ')
			col++
			for col%TabStop != 0 {
				b.WriteByte(' ')
				col++
			}
		} else {
			b.WriteByte(c)
			col++
		}
	}
	r.render = b.String()
}

// CxToRx converts a byte offset in the row to a display column.
func (r *Row) CxToRx(cx int) int {
	if cx > len(r.chars) {
		cx = len(r.chars)
	}
	rx := 0
	for i := 0; i < cx; i++ {
		if r.chars[i] == '\t' {
			rx += TabStop - (rx % TabStop)
		} else {
			rx++
		}
	}
	return rx
}

// RxToCx converts a display column back to a byte offset. A column inside a
// tab maps to the tab itself.
func (r *Row) RxToCx(rx int) int {
	curRx := 0
	for cx := 0; cx < len(r.chars); cx++ {
		if r.chars[cx] == '\t' {
			curRx += TabStop - (curRx % TabStop)
		} else {
			curRx++
		}
		if curRx > rx {
			return cx
		}
	}
	return len(r.chars)
}

// Buffer is the document.
type Buffer struct {
	rows []Row
}

// New returns an empty buffer with no rows.
func New() *Buffer {
	return &Buffer{}
}

// NumRows returns the number of rows.
func (b *Buffer) NumRows() int { return len(b.rows) }

// Row returns a pointer to row i, or nil if i is out of range.
func (b *Buffer) Row(i int) *Row {
	if i < 0 || i >= len(b.rows) {
		return nil
	}
	return &b.rows[i]
}

func (b *Buffer) appendRow(chars []byte) {
	r := Row{chars: chars}
	r.updateRender()
	b.rows = append(b.rows, r)
}

// InsertRow inserts a copy of s as a new row at index at.
func (b *Buffer) InsertRow(at int, s []byte) {
	if at < 0 {
		at = 0
	}
	if at > len(b.rows) {
		at = len(b.rows)
	}
	r := Row{chars: append([]byte(nil), s...)}
	r.updateRender()
	b.rows = append(b.rows, Row{})
	copy(b.rows[at+1:], b.rows[at:])
	b.rows[at] = r
}

// DeleteRow removes row at. Out-of-range indices are ignored.
func (b *Buffer) DeleteRow(at int) {
	if at < 0 || at >= len(b.rows) {
		return
	}
	b.rows = append(b.rows[:at], b.rows[at+1:]...)
}

// InsertChar inserts ru at (cy, cx). cy may be NumRows, which starts a new
// last line.
func (b *Buffer) InsertChar(cy, cx int, ru rune) {
	if cy < 0 {
		return
	}
	if cy == len(b.rows) {
		b.appendRow(nil)
	}
	if cy >= len(b.rows) {
		return
	}
	row := &b.rows[cy]
	if cx < 0 {
		cx = 0
	}
	if cx > len(row.chars) {
		cx = len(row.chars)
	}
	enc := []byte(string(ru))
	row.chars = append(row.chars, enc...)
	copy(row.chars[cx+len(enc):], row.chars[cx:])
	copy(row.chars[cx:], enc)
	row.updateRender()
}

// DeleteChar deletes the byte before (cy, cx), like Backspace. At the start of
// a line it joins the line onto the previous one.
func (b *Buffer) DeleteChar(cy, cx int) {
	if cy < 0 || cy >= len(b.rows) {
		return
	}
	if cx == 0 && cy == 0 {
		return
	}
	row := &b.rows[cy]
	if cx > 0 {
		if cx > len(row.chars) {
			cx = len(row.chars)
		}
		row.chars = append(row.chars[:cx-1], row.chars[cx:]...)
		row.updateRender()
		return
	}
	prev := &b.rows[cy-1]
	prev.chars = append(prev.chars, row.chars...)
	prev.updateRender()
	b.DeleteRow(cy)
}

// InsertNewline splits row cy at cx. Past the last row it appends an empty
// row.
func (b *Buffer) InsertNewline(cy, cx int) {
	if cy < 0 {
		return
	}
	if cy >= len(b.rows) {
		b.InsertRow(len(b.rows), nil)
		return
	}
	row := &b.rows[cy]
	if cx < 0 {
		cx = 0
	}
	if cx > len(row.chars) {
		cx = len(row.chars)
	}
	if cx == 0 {
		b.InsertRow(cy, nil)
		return
	}
	tail := append([]byte(nil), row.chars[cx:]...)
	b.InsertRow(cy+1, tail)
	// InsertRow may have reallocated b.rows.
	row = &b.rows[cy]
	row.chars = row.chars[:cx]
	row.updateRender()
}

// Load reads a document, accepting LF or CRLF line endings. A final newline
// does not add an empty last row.
func Load(r io.Reader) (*Buffer, error) {
	b := New()
	br := bufio.NewReader(r)
	for {
		chunk, err := br.ReadBytes('\n')
		if len(chunk) > 0 {
			line := bytes.TrimSuffix(chunk, []byte{'\n'})
			line = bytes.TrimSuffix(line, []byte{'\r'})
			b.appendRow(append([]byte(nil), line...))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// Save writes each row followed by '\n' and returns the byte count.
func (b *Buffer) Save(w io.Writer) (int, error) {
	total := 0
	for i := range b.rows {
		n, err := w.Write(b.rows[i].chars)
		total += n
		if err != nil {
			return total, err
		}
		n, err = w.Write([]byte{'\n'})
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// Bytes returns what Save would write.
func (b *Buffer) Bytes() []byte {
	var buf bytes.Buffer
	_, _ = b.Save(&buf)
	return buf.Bytes()
}
