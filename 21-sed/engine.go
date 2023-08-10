package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// State is the pattern space plus the flags the line cycle needs.
type State struct {
	Line    string // the pattern space
	LineNo  int    // 1-based
	IsLast  bool
	Quiet   bool
	Deleted bool // set by `d`
	Subst   bool // an `s` command changed the line
	Out     *bufio.Writer
}

type Engine struct {
	Commands []*Command
	Quiet    bool
}

// reset clears range state so the engine can be reused for the next file
// under -i.
func (e *Engine) reset() {
	for _, c := range e.Commands {
		if r, ok := c.Addr.(*rangeAddr); ok {
			r.active = false
		}
	}
}

// readLine returns one line without its newline (or CRLF), whether anything
// was read, and whether the line ended in a newline.
func readLine(br *bufio.Reader) (line string, ok bool, hadNL bool, err error) {
	s, e := br.ReadString('\n')
	if len(s) == 0 {
		return "", false, false, e
	}
	if strings.HasSuffix(s, "\n") {
		hadNL = true
		s = s[:len(s)-1]
		// Normalize CRLF: strip a trailing CR so regexes such as /^$/ and $
		// anchors behave the same on Windows-style input.
		if strings.HasSuffix(s, "\r") {
			s = s[:len(s)-1]
		}
	}
	if e == io.EOF {
		e = nil // report EOF on the next call instead
	}
	return s, true, hadNL, e
}

func (e *Engine) Run(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	st := &State{Quiet: e.Quiet, Out: bw}

	cur, ok, curNL, err := readLine(br)
	if err != nil && err != io.EOF {
		return err
	}
	lineNo := 0
	for ok {
		// One line of lookahead lets us know when cur is the last line ($).
		next, nextOk, nextNL, nerr := readLine(br)
		if nerr != nil && nerr != io.EOF {
			return nerr
		}
		lineNo++

		st.Line = cur
		st.LineNo = lineNo
		st.IsLast = !nextOk
		st.Deleted = false
		st.Subst = false

		for _, c := range e.Commands {
			if c.matches(st) {
				c.Action.Execute(st)
				if st.Deleted {
					break
				}
			}
		}

		if !st.Quiet && !st.Deleted {
			bw.WriteString(st.Line)
			// Don't add a newline the input's last line didn't have.
			if curNL {
				bw.WriteByte('\n')
			}
		}

		cur, ok, curNL = next, nextOk, nextNL
	}
	return bw.Flush()
}

// RunInPlace writes to a temp file next to path and renames it over the
// original. A non-empty suffix keeps the original as path+suffix.
func (e *Engine) RunInPlace(path, suffix string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ccsed-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	e.reset()
	if err := e.Run(in, tmp); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	in.Close()

	// Best-effort preservation of the original file mode.
	if fi, serr := os.Stat(path); serr == nil {
		os.Chmod(tmpName, fi.Mode())
	}

	if suffix != "" {
		if err := os.Rename(path, path+suffix); err != nil {
			os.Remove(tmpName)
			return err
		}
	}
	return os.Rename(tmpName, path)
}
