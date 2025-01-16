package main

import "fmt"

// Tape is the Brainfuck memory. Cells are uint8, so arithmetic wraps on its
// own. Moving off either end is an error unless wrap is set, in which case
// the tape is circular.
type Tape struct {
	cells []uint8
	dp    int
	wrap  bool
}

func NewTape(size int, wrap bool) *Tape {
	if size < 1 {
		size = 1
	}
	return &Tape{cells: make([]uint8, size), wrap: wrap}
}

func (t *Tape) get() uint8 { return t.cells[t.dp] }

func (t *Tape) set(v uint8) { t.cells[t.dp] = v }

func (t *Tape) add(delta int) {
	t.cells[t.dp] += uint8(delta)
}

// index checks i against the tape, wrapping it if the tape is circular.
func (t *Tape) index(i int) (int, error) {
	m := len(t.cells)
	if i >= 0 && i < m {
		return i, nil
	}
	if !t.wrap {
		return 0, &RuntimeError{Msg: fmt.Sprintf("data pointer out of bounds: %d (tape size %d)", i, m)}
	}
	return (i%m + m) % m, nil
}

func (t *Tape) move(n int) error {
	i, err := t.index(t.dp + n)
	if err != nil {
		return err
	}
	t.dp = i
	return nil
}

// at returns the cell offset cells away from the pointer, for OpAddTo.
func (t *Tape) at(offset int) (*uint8, error) {
	i, err := t.index(t.dp + offset)
	if err != nil {
		return nil, err
	}
	return &t.cells[i], nil
}
