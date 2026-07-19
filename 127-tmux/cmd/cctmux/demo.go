package main

import (
	"fmt"
	"strings"
	"time"

	"cctmux/internal/input"
	"cctmux/internal/pty"
	"cctmux/internal/render"
	"cctmux/internal/session"
)

// runDemo drives a session with mock PTYs through a few splits and window
// switches and prints each frame, so the rendering can be seen on platforms
// without real PTYs.
func runDemo() error {
	const w, h = 60, 16
	f := func(cw, ch int) (pty.PTY, error) { return pty.NewMock(cw, ch, true), nil }
	m := session.NewManager(f, w, h)
	s, err := m.Create("demo")
	if err != nil {
		return err
	}
	disp := input.New(s, 0)

	feed(disp, s, "echo hello from cctmux\r")

	feed(disp, s, string([]byte{input.DefaultPrefix, '%'})) // split left/right
	feed(disp, s, "ls -la\r")

	feed(disp, s, string([]byte{input.DefaultPrefix, '"'})) // split right pane top/bottom
	feed(disp, s, "top\r")

	printFrame("Three panes (vertical + horizontal split)", s, w, h)

	feed(disp, s, string([]byte{input.DefaultPrefix, 'c'})) // new window
	feed(disp, s, "vim notes.md\r")
	printFrame("Second window active (status bar shows 0 and 1)", s, w, h)

	feed(disp, s, string([]byte{input.DefaultPrefix, '0'}))
	printFrame("Back on window 0", s, w, h)

	return nil
}

func feed(d *input.Dispatcher, s *session.Session, keys string) {
	for _, b := range []byte(keys) {
		res := d.Dispatch(b)
		if len(res.Forward) > 0 {
			if p := s.ActiveWindow().ActivePane(); p != nil {
				p.PTY.Write(res.Forward)
			}
		}
	}
	drain(s)
}

// drain copies whatever the mock shells have echoed into their pane screens.
// The real server does this from a goroutine per pane.
func drain(s *session.Session) {
	buf := make([]byte, 4096)
	for _, win := range s.Windows() {
		for _, p := range win.Panes() {
			mock, ok := p.PTY.(*pty.Mock)
			if !ok {
				continue
			}
			for {
				n, _ := mock.ReadNonBlock(buf)
				if n == 0 {
					break
				}
				p.Screen.Write(buf[:n])
			}
		}
	}
}

func printFrame(title string, s *session.Session, w, h int) {
	grid := render.Render(s, w, h, w, time.Now().Format("15:04:05"))
	fmt.Println()
	fmt.Println("== " + title + " ==")
	fmt.Println("+" + strings.Repeat("-", w) + "+")
	for y := 0; y < h; y++ {
		line := []rune(grid.Line(y)) // trailing spaces are trimmed; pad back out
		row := []rune(strings.Repeat(" ", w))
		copy(row, line)
		fmt.Println("|" + string(row) + "|")
	}
	fmt.Println("+" + strings.Repeat("-", w) + "+")
}
