package comm

import (
	"bufio"
	"io"
)

// lineSource reads one line at a time and keeps only the current one. A last
// line with no trailing newline still counts. advance also reports whether
// the input went out of order, so the order check needs no extra buffering.
type lineSource struct {
	r      *bufio.Reader
	fileNo int // 1 or 2, for messages
	ci     bool
	cur    []byte
	has    bool // false once input is exhausted
}

// newLineSource wraps rd. Call start before using it.
func newLineSource(rd io.Reader, fileNo int, ci bool) *lineSource {
	return &lineSource{r: bufio.NewReader(rd), fileNo: fileNo, ci: ci}
}

// readLine returns the next line without its '\n'. ok is false at EOF.
func (s *lineSource) readLine() (line []byte, ok bool, err error) {
	line, err = s.r.ReadBytes('\n')
	if err == io.EOF {
		if len(line) == 0 {
			return nil, false, nil
		}
		return line, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return line[:len(line)-1], true, nil
}

func (s *lineSource) start() error {
	line, ok, err := s.readLine()
	if err != nil {
		return err
	}
	s.cur, s.has = line, ok
	return nil
}

func (s *lineSource) current() ([]byte, bool) {
	return s.cur, s.has
}

// advance moves to the next line. outOfOrder is true if the previous line
// sorts after the new one.
func (s *lineSource) advance() (outOfOrder bool, err error) {
	prev, hadPrev := s.cur, s.has
	line, ok, err := s.readLine()
	if err != nil {
		return false, err
	}
	s.cur, s.has = line, ok
	return hadPrev && ok && compareLines(prev, line, s.ci) > 0, nil
}
