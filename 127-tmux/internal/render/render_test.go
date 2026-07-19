package render

import (
	"strings"
	"testing"

	"cctmux/internal/layout"
	"cctmux/internal/pty"
	"cctmux/internal/session"
)

func factory() session.PTYFactory {
	return func(w, h int) (pty.PTY, error) { return pty.NewMock(w, h, true), nil }
}

func newSession(t *testing.T, w, h int) *session.Session {
	t.Helper()
	m := session.NewManager(factory(), w, h)
	s, err := m.Create("dev")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStatusBarLayout(t *testing.T) {
	s := newSession(t, 40, 10)
	bar := StatusBar(s, 40, "12:00")
	if !strings.HasPrefix(bar, "[dev]") {
		t.Fatalf("status should start with session name: %q", bar)
	}
	if !strings.HasSuffix(strings.TrimRight(bar, " "), "12:00") {
		t.Fatalf("status should end with clock: %q", bar)
	}
	if !strings.Contains(bar, "0:bash*") {
		t.Fatalf("status should show active window marker: %q", bar)
	}
	if len([]rune(bar)) != 40 {
		t.Fatalf("status width = %d", len([]rune(bar)))
	}
}

func TestRenderSinglePaneContent(t *testing.T) {
	s := newSession(t, 20, 6)
	p := s.ActiveWindow().ActivePane()
	p.Screen.Write([]byte("hello world"))
	grid := Render(s, 20, 6, 20, "09:41")
	if grid.Line(0) != "hello world" {
		t.Fatalf("row0 = %q", grid.Line(0))
	}
	// Status bar on the last row.
	if !strings.Contains(grid.Line(5), "[dev]") {
		t.Fatalf("status row = %q", grid.Line(5))
	}
}

func TestRenderVerticalSplitHasBorder(t *testing.T) {
	s := newSession(t, 21, 6) // 20 usable cols -> border at col 10
	s.SplitActive(layout.Vertical)
	grid := Render(s, 21, 6, 21, "00:00")
	// There should be a vertical border glyph somewhere on row 0.
	found := false
	for x := 0; x < 21; x++ {
		if grid.Cell(x, 0).Rune == glyphV {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a vertical border on row 0: %q", grid.Line(0))
	}
}

func TestActivePaneBorderHighlighted(t *testing.T) {
	s := newSession(t, 21, 6)
	s.SplitActive(layout.Vertical) // new right pane is active
	grid := Render(s, 21, 6, 21, "00:00")
	highlighted := false
	for x := 0; x < 21; x++ {
		c := grid.Cell(x, 0)
		if c.Rune == glyphV && c.Style == activeStyle {
			highlighted = true
		}
	}
	if !highlighted {
		t.Fatal("active pane border should use the highlight style")
	}
}

func TestRenderHorizontalSplitContentBothPanes(t *testing.T) {
	s := newSession(t, 12, 8)
	top := s.ActiveWindow().ActivePane()
	top.Screen.Write([]byte("TOP"))
	id, _ := s.SplitActive(layout.Horizontal)
	bottom := s.ActiveWindow().Pane(id)
	bottom.Screen.Write([]byte("BOT"))
	grid := Render(s, 12, 8, 12, "00:00")
	if grid.Line(0) != "TOP" {
		t.Fatalf("top pane row = %q", grid.Line(0))
	}
	// The bottom pane content must appear on some row below the divider.
	joined := ""
	for y := 1; y < 7; y++ {
		joined += grid.Line(y) + "\n"
	}
	if !strings.Contains(joined, "BOT") {
		t.Fatalf("bottom pane content missing:\n%s", joined)
	}
}

func TestCopyModeBanner(t *testing.T) {
	s := newSession(t, 20, 6)
	p := s.ActiveWindow().ActivePane()
	for i := 0; i < 10; i++ {
		p.Screen.Write([]byte("line\r\n"))
	}
	p.Copy = true
	p.ScrollOff = 3
	grid := Render(s, 20, 6, 20, "00:00")
	if !strings.Contains(grid.Line(0), "COPY") {
		t.Fatalf("copy banner missing on row0: %q", grid.Line(0))
	}
}
