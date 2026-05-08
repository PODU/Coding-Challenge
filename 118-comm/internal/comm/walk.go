package comm

import "bufio"

// counts are the per-column line totals for --total.
type counts struct {
	col1 int // unique to FILE1
	col2 int // unique to FILE2
	col3 int // common to both
}

type columnWriter struct {
	w         *bufio.Writer
	delimiter string
	suppress  [4]bool
}

// emit writes line in column col (1-3), indented by one delimiter for each
// visible column to its left. Suppressed columns print nothing.
func (cw *columnWriter) emit(col int, line []byte) error {
	if cw.suppress[col] {
		return nil
	}
	for i := 1; i < col; i++ {
		if cw.suppress[i] {
			continue
		}
		if _, err := cw.w.WriteString(cw.delimiter); err != nil {
			return err
		}
	}
	if _, err := cw.w.Write(line); err != nil {
		return err
	}
	return cw.w.WriteByte('\n')
}

// orderHandler is called when an input is out of order. Returning an error
// stops the walk.
type orderHandler func(fileNo int) error

// mergeWalk merges the two sorted inputs, writing each line to its column.
func mergeWalk(a, b *lineSource, cw *columnWriter, ci bool, onOrder orderHandler) (counts, error) {
	var c counts

	advance := func(s *lineSource) error {
		violated, err := s.advance()
		if err != nil {
			return err
		}
		if violated {
			return onOrder(s.fileNo)
		}
		return nil
	}

	for {
		la, oka := a.current()
		lb, okb := b.current()

		var cmp int
		switch {
		case !oka && !okb:
			return c, nil
		case !okb:
			cmp = -1
		case !oka:
			cmp = 1
		default:
			cmp = compareLines(la, lb, ci)
		}

		var err error
		switch {
		case cmp < 0:
			if err := cw.emit(1, la); err != nil {
				return c, err
			}
			c.col1++
			err = advance(a)
		case cmp > 0:
			if err := cw.emit(2, lb); err != nil {
				return c, err
			}
			c.col2++
			err = advance(b)
		default:
			// With -i the two lines may differ in case; print FILE1's.
			if err := cw.emit(3, la); err != nil {
				return c, err
			}
			c.col3++
			if err = advance(a); err == nil {
				err = advance(b)
			}
		}
		if err != nil {
			return c, err
		}
	}
}
