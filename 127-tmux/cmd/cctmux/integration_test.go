package main

import (
	"strings"
	"testing"

	"cctmux/internal/command"
	"cctmux/internal/input"
	"cctmux/internal/pty"
	"cctmux/internal/render"
	"cctmux/internal/session"
)

func mockFactory() session.PTYFactory {
	return func(w, h int) (pty.PTY, error) { return pty.NewMock(w, h, true), nil }
}

// pump copies mock echo output into pane screens, mimicking the server's reader.
func pump(s *session.Session) {
	buf := make([]byte, 4096)
	for _, win := range s.Windows() {
		for _, p := range win.Panes() {
			m := p.PTY.(*pty.Mock)
			for m.Available() > 0 {
				n, _ := m.ReadNonBlock(buf)
				if n == 0 {
					break
				}
				p.Screen.Write(buf[:n])
			}
		}
	}
}

func typeKeys(d *input.Dispatcher, s *session.Session, keys string) {
	for _, b := range []byte(keys) {
		r := d.Dispatch(b)
		if len(r.Forward) > 0 {
			s.ActiveWindow().ActivePane().PTY.Write(r.Forward)
		}
	}
	pump(s)
}

func TestDemoRunsClean(t *testing.T) {
	if err := runDemo(); err != nil {
		t.Fatalf("demo failed: %v", err)
	}
}

// Steps 3-6: sessions persist across "detach", reattach shows prior content,
// list and kill behave, and the server would shut down when empty.
func TestDetachReattachPersistence(t *testing.T) {
	m := session.NewManager(mockFactory(), 40, 12)
	s, err := m.Create("work")
	if err != nil {
		t.Fatal(err)
	}
	d := input.New(s, 0)
	typeKeys(d, s, "echo persisted\r")

	// "Detach": the client goes away but the session stays in the manager.
	s.Attached = false
	if m.Get("work") == nil {
		t.Fatal("session should persist after detach")
	}

	// Reattach: same session, content still present.
	s2 := m.Get("work")
	grid := render.Render(s2, 40, 12, 40, "00:00")
	if !strings.Contains(grid.Line(0), "echo persisted") {
		t.Fatalf("reattach lost content: %q", grid.Line(0))
	}
}

func TestListAndKillLifecycle(t *testing.T) {
	m := session.NewManager(mockFactory(), 40, 12)
	m.Create("a")
	s, _ := m.Create("b")

	res, err := command.Exec(m, s, "list-sessions")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "a: 1 window") || !strings.Contains(res.Output, "b: 1 window") {
		t.Fatalf("list output = %q", res.Output)
	}

	// Kill both; the manager becoming empty is what triggers server shutdown.
	if r, _ := command.Exec(m, s, "kill-session -t a"); r.KillSession == "a" {
		m.Kill("a")
	}
	if r, _ := command.Exec(m, s, "kill-session -t b"); r.KillSession == "b" {
		m.Kill("b")
	}
	if !m.Empty() {
		t.Fatal("manager should be empty after killing all sessions")
	}
}

// Step 8-9 through the real prefix path: create windows and navigate.
func TestWindowWorkflowViaPrefix(t *testing.T) {
	m := session.NewManager(mockFactory(), 40, 12)
	s, _ := m.Create("s")
	d := input.New(s, 0)
	typeKeys(d, s, string([]byte{input.DefaultPrefix, 'c'})) // window 1
	typeKeys(d, s, string([]byte{input.DefaultPrefix, 'c'})) // window 2
	if s.WindowCount() != 3 {
		t.Fatalf("want 3 windows, got %d", s.WindowCount())
	}
	grid := render.Render(s, 40, 12, 40, "00:00")
	status := grid.Line(11)
	if !strings.Contains(status, "2:bash*") {
		t.Fatalf("status should mark window 2 active: %q", status)
	}
}
