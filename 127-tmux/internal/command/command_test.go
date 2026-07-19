package command

import (
	"strings"
	"testing"

	"cctmux/internal/pty"
	"cctmux/internal/session"
)

func mgr(t *testing.T) (*session.Manager, *session.Session) {
	t.Helper()
	f := func(w, h int) (pty.PTY, error) { return pty.NewMock(w, h, true), nil }
	m := session.NewManager(f, 80, 24)
	s, err := m.Create("main")
	if err != nil {
		t.Fatal(err)
	}
	return m, s
}

func TestNewWindow(t *testing.T) {
	m, s := mgr(t)
	if _, err := Exec(m, s, "new-window -n logs"); err != nil {
		t.Fatal(err)
	}
	if s.WindowCount() != 2 {
		t.Fatalf("want 2 windows, got %d", s.WindowCount())
	}
	if s.ActiveWindow().Name != "logs" {
		t.Fatalf("name = %q", s.ActiveWindow().Name)
	}
}

func TestSplitHorizontalVertical(t *testing.T) {
	m, s := mgr(t)
	if _, err := Exec(m, s, "split-window -h"); err != nil {
		t.Fatal(err)
	}
	if len(s.ActiveWindow().Panes()) != 2 {
		t.Fatalf("split -h should create a pane")
	}
	if _, err := Exec(m, s, "split-window -v"); err != nil {
		t.Fatal(err)
	}
	if len(s.ActiveWindow().Panes()) != 3 {
		t.Fatalf("split -v should create a third pane")
	}
}

func TestSelectAndCycleWindows(t *testing.T) {
	m, s := mgr(t)
	Exec(m, s, "new-window -n a")
	Exec(m, s, "new-window -n b") // index 2 active
	if _, err := Exec(m, s, "select-window -t 0"); err != nil {
		t.Fatal(err)
	}
	if s.ActiveIndex() != 0 {
		t.Fatalf("select-window active = %d", s.ActiveIndex())
	}
	Exec(m, s, "next-window")
	if s.ActiveIndex() != 1 {
		t.Fatalf("next-window active = %d", s.ActiveIndex())
	}
	Exec(m, s, "previous-window")
	if s.ActiveIndex() != 0 {
		t.Fatalf("previous-window active = %d", s.ActiveIndex())
	}
	if _, err := Exec(m, s, "select-window -t 9"); err == nil {
		t.Fatal("out-of-range select should error")
	}
}

func TestRenameWindowAndSession(t *testing.T) {
	m, s := mgr(t)
	if _, err := Exec(m, s, "rename-window build"); err != nil {
		t.Fatal(err)
	}
	if s.ActiveWindow().Name != "build" {
		t.Fatalf("window name = %q", s.ActiveWindow().Name)
	}
	if _, err := Exec(m, s, "rename-session dev"); err != nil {
		t.Fatal(err)
	}
	if m.Get("dev") == nil || m.Get("main") != nil {
		t.Fatal("session should have been renamed to dev")
	}
}

func TestKillSession(t *testing.T) {
	m, s := mgr(t)
	m.Create("other")
	res, err := Exec(m, s, "kill-session -t other")
	if err != nil {
		t.Fatal(err)
	}
	if res.KillSession != "other" {
		t.Fatalf("KillSession = %q", res.KillSession)
	}
	// Default target is the current session.
	res, _ = Exec(m, s, "kill-session")
	if res.KillSession != "main" {
		t.Fatalf("default kill target = %q", res.KillSession)
	}
	if _, err := Exec(m, s, "kill-session -t ghost"); err == nil {
		t.Fatal("killing missing session should error")
	}
}

func TestListSessions(t *testing.T) {
	m, s := mgr(t)
	m.Create("zeta")
	res, err := Exec(m, s, "list-sessions")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "main:") || !strings.Contains(res.Output, "zeta:") {
		t.Fatalf("list output = %q", res.Output)
	}
}

func TestResizeAndSelectPane(t *testing.T) {
	m, s := mgr(t)
	Exec(m, s, "split-window -h") // two side-by-side panes, right active
	if _, err := Exec(m, s, "resize-pane -L 3"); err != nil {
		t.Fatal(err)
	}
	// Move focus left to the first pane.
	if _, err := Exec(m, s, "select-pane -L"); err != nil {
		t.Fatal(err)
	}
	if s.ActiveWindow().Layout.Active() != 0 {
		t.Fatalf("select-pane -L should focus pane 0")
	}
}

func TestDetachAndUnknown(t *testing.T) {
	m, s := mgr(t)
	res, err := Exec(m, s, "detach")
	if err != nil || !res.Detach {
		t.Fatalf("detach: res=%+v err=%v", res, err)
	}
	if _, err := Exec(m, s, "frobnicate"); err == nil {
		t.Fatal("unknown command should error")
	}
}

func TestEmptyLineNoop(t *testing.T) {
	m, s := mgr(t)
	if res, err := Exec(m, s, "   "); err != nil || res != (Result{}) {
		t.Fatalf("empty line: res=%+v err=%v", res, err)
	}
}
