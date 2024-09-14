// Command editor is a small modal text editor for the terminal.
package main

import (
	"fmt"
	"os"

	"github.com/example/editor/buffer"
	"github.com/example/editor/editor"
	"github.com/example/editor/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "editor:", err)
		os.Exit(1)
	}
}

func run() error {
	var filename string
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}

	buf, err := loadBuffer(filename)
	if err != nil {
		return err
	}

	t, err := term.Open()
	if err != nil {
		return err
	}
	// Deferring Close means even a panic restores the terminal.
	defer t.Close()

	rows, cols := t.Size()
	ed := editor.New(buf, rows, cols)
	if filename != "" {
		ed.SetFilename(filename)
	}
	ed.SetStatus("HELP: :w save | :q quit | :wq save+quit | i insert | / search")

	for {
		// Picks up window resizes, at the latest on the next keypress.
		rows, cols = t.Size()
		ed.SetSize(rows, cols)

		if err := t.Draw(toTermFrame(ed.Render())); err != nil {
			return err
		}

		key, err := t.ReadKey()
		if err != nil {
			return err
		}
		if ed.HandleKey(key) {
			return nil
		}
	}
}

// loadBuffer returns an empty buffer for a file that doesn't exist yet; saving
// will create it.
func loadBuffer(filename string) (*buffer.Buffer, error) {
	if filename == "" {
		return buffer.New(), nil
	}
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return buffer.New(), nil
		}
		return nil, err
	}
	defer f.Close()
	return buffer.Load(f)
}

func toTermFrame(f editor.Frame) term.Frame {
	return term.Frame{
		Lines:     f.Lines,
		CursorRow: f.CursorRow,
		CursorCol: f.CursorCol,
		Status:    f.Status,
		Message:   f.Message,
	}
}
