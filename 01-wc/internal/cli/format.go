package cli

import (
	"fmt"
	"strings"

	"ccwc/internal/counter"
)

func digits(n int64) int {
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}

// defaultWidth is what GNU wc uses when an input's size isn't known up front
// (stdin, pipes).
const defaultWidth = 7

// FieldWidth picks the column width the way GNU wc does. A single count for a
// single input is unpadded. If every input is a regular file, the width fits
// the total byte count, which is always the largest number printed. Otherwise
// it falls back to 7.
func FieldWidth(selectedCount, numInputs int, allSizesKnown bool, totalBytes int64) int {
	if selectedCount == 1 && numInputs <= 1 {
		return 1
	}
	if allSizesKnown {
		return digits(totalBytes)
	}
	return defaultWidth
}

// SelectedCount returns how many count columns the options request.
func SelectedCount(o Options) int {
	n := 0
	for _, b := range []bool{o.Lines, o.Words, o.Chars, o.Bytes} {
		if b {
			n++
		}
	}
	return n
}

// FormatRow renders one output line. Columns always come in the order lines,
// words, chars, bytes, right-aligned to width. An empty label is omitted.
func FormatRow(o Options, c counter.Counts, label string, width int) string {
	var sb strings.Builder
	first := true
	writeCol := func(v int64) {
		if !first {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%*d", width, v)
		first = false
	}

	if o.Lines {
		writeCol(c.Lines)
	}
	if o.Words {
		writeCol(c.Words)
	}
	if o.Chars {
		writeCol(c.Chars)
	}
	if o.Bytes {
		writeCol(c.Bytes)
	}

	if label != "" {
		sb.WriteByte(' ')
		sb.WriteString(label)
	}
	return sb.String()
}
