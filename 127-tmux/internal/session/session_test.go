package session

import (
	"testing"

	"cctmux/internal/layout"
	"cctmux/internal/pty"
)

// mockFactory builds echo-mock PTYs and records the last size requested.
func mockFactory() PTYFactory {
	return func(w, h int) (pty.PTY, error) {
		return pty.NewMock(w, h, true), nil
	}
}

func newMgr(t *testing.T) *Manager {
	t.Helper()
	return NewManager(mockFactory(), 80, 24)
}

func TestCreateSessionAndDuplicate(t *testing.T) {
	m := newMgr(t)
	if _, err := m.Create("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create("work"); err == nil {
		t.Fatal("duplicate name should error")
	}
	if m.Count() != 1 {
		t.Fatalf("count = %d", m.Count())
	}
	s := m.Get("work")
	if s == nil || s.WindowCount() != 1 {
		t.Fatalf("session missing or wrong window count")
	}
	if s.ActiveWindow().ActivePane() == nil {
		t.Fatal("session should start with one pane")
	}
}

func TestKillAndEmpty(t *testing.T) {
	m := newMgr(t)
	m.Create("a")
	m.Create("b")
	if !m.Kill("a") {
		t.Fatal("kill a failed")
	}
	if m.Kill("a") {
		t.Fatal("double kill should fail")
	}
	if m.Empty() {
		t.Fatal("still has b")
	}
	m.Kill("b")
	if !m.Empty() {
		t.Fatal("should be empty")
	}
}

func TestListFormat(t *testing.T) {
	m := newMgr(t)
	m.Create("alpha")
	s, _ := m.Create("beta")
	s.Attached = true
	s.NewWindow("editor") // beta now has 2 windows
	out := m.FormatList()
	want := "alpha: 1 window\nbeta: 2 windows (attached)\n"
	if out != want {
		t.Fatalf("list =\n%q\nwant\n%q", out, want)
	}
}

func TestNewWindowNavigation(t *testing.T) {
	m := newMgr(t)
	s, _ := m.Create("s")
	s.NewWindow("two")   // index 1 active
	s.NewWindow("three") // index 2 active
	if s.ActiveIndex() != 2 {
		t.Fatalf("active = %d", s.ActiveIndex())
	}
	s.NextWindow() // wrap to 0
	if s.ActiveIndex() != 0 {
		t.Fatalf("next-wrap active = %d", s.ActiveIndex())
	}
	s.PrevWindow() // wrap back to 2
	if s.ActiveIndex() != 2 {
		t.Fatalf("prev-wrap active = %d", s.ActiveIndex())
	}
	if !s.SelectWindow(1) || s.ActiveIndex() != 1 {
		t.Fatalf("select window 1 failed")
	}
	if s.SelectWindow(9) {
		t.Fatal("selecting out-of-range window should fail")
	}
}

func TestSplitAndPaneSizing(t *testing.T) {
	m := newMgr(t)
	s, _ := m.Create("s")
	win := s.ActiveWindow()
	// Full pane before split: 80 wide, 23 tall (24 - status bar).
	full := win.ActivePane()
	fw, fh := full.Screen.Size()
	if fw != 80 || fh != 23 {
		t.Fatalf("full pane size = %dx%d", fw, fh)
	}
	id, err := s.SplitActive(layout.Vertical)
	if err != nil {
		t.Fatal(err)
	}
	if len(win.Panes()) != 2 {
		t.Fatalf("want 2 panes, got %d", len(win.Panes()))
	}
	// Each pane should be about half width (80-1 divider => 40/39 area) and the
	// PTY should have been notified.
	np := win.Pane(id)
	pw, _ := np.Screen.Size()
	if pw >= 80 || pw < 30 {
		t.Fatalf("new pane width after vsplit = %d", pw)
	}
	mp := np.PTY.(*pty.Mock)
	if mp.Resizes() == 0 {
		t.Fatal("split should resize the new pane's PTY")
	}
}

func TestCloseLastPaneKillsWindow(t *testing.T) {
	m := newMgr(t)
	s, _ := m.Create("s")
	s.NewWindow("two") // 2 windows now, active index 1
	// Close the single pane of window 1 -> window removed, session survives.
	if !s.CloseActivePane() {
		t.Fatal("session should survive closing a pane while another window exists")
	}
	if s.WindowCount() != 1 {
		t.Fatalf("want 1 window left, got %d", s.WindowCount())
	}
}

func TestCloseLastPaneOfLastWindow(t *testing.T) {
	m := newMgr(t)
	s, _ := m.Create("s")
	if s.CloseActivePane() {
		t.Fatal("closing the only pane of the only window should report session death")
	}
}

func TestResizeUpdatesPanes(t *testing.T) {
	m := newMgr(t)
	s, _ := m.Create("s")
	s.Resize(100, 30)
	p := s.ActiveWindow().ActivePane()
	w, h := p.Screen.Size()
	if w != 100 || h != 29 {
		t.Fatalf("after resize pane = %dx%d", w, h)
	}
}

func TestZoomTogglesLayout(t *testing.T) {
	m := newMgr(t)
	s, _ := m.Create("s")
	s.SplitActive(layout.Horizontal)
	s.ToggleZoom()
	if !s.ActiveWindow().Layout.Zoomed() {
		t.Fatal("expected zoom on")
	}
	zoomed := s.ActiveWindow().ActivePane()
	_, h := zoomed.Screen.Size()
	if h != 23 {
		t.Fatalf("zoomed pane should fill area height 23, got %d", h)
	}
}
