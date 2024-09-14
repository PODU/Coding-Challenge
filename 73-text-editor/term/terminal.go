package term

import (
	"fmt"
	"io"
	"os"
	"strings"

	xterm "golang.org/x/term"
)

// Terminal is the real terminal, in raw mode.
type Terminal struct {
	in        *os.File
	out       *os.File
	fd        int
	prevState *xterm.State
	restoreVT func()
	*KeyReader
}

// Open switches the terminal on stdin to raw mode (and, on Windows, turns on
// ANSI escape handling). Defer Close straight away so that even a panic puts
// the terminal back.
func Open() (*Terminal, error) {
	in, out := os.Stdin, os.Stdout
	fd := int(in.Fd())

	state, err := xterm.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("entering raw mode: %w", err)
	}
	return &Terminal{
		in:        in,
		out:       out,
		fd:        fd,
		prevState: state,
		restoreVT: enableVirtualTerminal(out),
		KeyReader: NewKeyReader(in),
	}, nil
}

// Close clears the screen and restores the terminal's previous mode.
func (t *Terminal) Close() {
	if t == nil {
		return
	}
	_, _ = io.WriteString(t.out, "\x1b[?25h\x1b[2J\x1b[H")
	if t.prevState != nil {
		_ = xterm.Restore(t.fd, t.prevState)
		t.prevState = nil
	}
	if t.restoreVT != nil {
		t.restoreVT()
		t.restoreVT = nil
	}
}

// Size returns the window size, or 24x80 if it can't be read.
func (t *Terminal) Size() (rows, cols int) {
	w, h, err := xterm.GetSize(t.fd)
	if err != nil || w <= 0 || h <= 0 {
		return 24, 80
	}
	return h, w
}

// Frame mirrors editor.Frame, so that term doesn't depend on editor.
type Frame struct {
	Lines     []string
	CursorRow int
	CursorCol int
	Status    string
	Message   string
}

// Draw writes the whole frame in one go to avoid flicker.
func (t *Terminal) Draw(f Frame) error {
	var b strings.Builder
	b.WriteString("\x1b[?25l") // hide cursor
	b.WriteString("\x1b[H")    // home

	for _, line := range f.Lines {
		b.WriteString(line)
		b.WriteString("\x1b[K") // clear to EOL
		b.WriteString("\r\n")
	}

	b.WriteString("\x1b[7m") // inverse video
	b.WriteString(f.Status)
	b.WriteString("\x1b[K")
	b.WriteString("\x1b[m")
	b.WriteString("\r\n")

	b.WriteString("\x1b[K")
	b.WriteString(f.Message)

	// ANSI cursor positions are 1-based.
	fmt.Fprintf(&b, "\x1b[%d;%dH", max(f.CursorRow, 0)+1, max(f.CursorCol, 0)+1)
	b.WriteString("\x1b[?25h") // show cursor

	_, err := io.WriteString(t.out, b.String())
	return err
}
