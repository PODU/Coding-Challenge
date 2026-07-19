package input

import (
	"bytes"
	"testing"

	"cctmux/internal/layout"
	"cctmux/internal/pty"
	"cctmux/internal/session"
)

func newSession(t *testing.T) *session.Session {
	t.Helper()
	f := func(w, h int) (pty.PTY, error) { return pty.NewMock(w, h, true), nil }
	m := session.NewManager(f, 80, 24)
	s, err := m.Create("s")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// feed pushes a byte sequence through the dispatcher and returns the aggregated
// forwarded bytes plus the last Result.
func feed(d *Dispatcher, data []byte) ([]byte, Result) {
	var fwd []byte
	var last Result
	for _, b := range data {
		r := d.Dispatch(b)
		fwd = append(fwd, r.Forward...)
		last = r
	}
	return fwd, last
}

func TestPlainKeysForwarded(t *testing.T) {
	d := New(newSession(t), 0)
	fwd, _ := feed(d, []byte("ls -la\r"))
	if string(fwd) != "ls -la\r" {
		t.Fatalf("forwarded = %q", fwd)
	}
}

func TestPrefixThenDetach(t *testing.T) {
	d := New(newSession(t), 0)
	_, res := feed(d, []byte{DefaultPrefix, 'd'})
	if !res.Detach {
		t.Fatal("prefix+d should detach")
	}
}

func TestPrefixTwiceSendsLiteralPrefix(t *testing.T) {
	d := New(newSession(t), 0)
	fwd, _ := feed(d, []byte{DefaultPrefix, DefaultPrefix})
	if !bytes.Equal(fwd, []byte{DefaultPrefix}) {
		t.Fatalf("prefix twice should forward one prefix byte, got %v", fwd)
	}
}

func TestPrefixNewWindow(t *testing.T) {
	s := newSession(t)
	d := New(s, 0)
	feed(d, []byte{DefaultPrefix, 'c'})
	if s.WindowCount() != 2 {
		t.Fatalf("prefix+c should create a window, count=%d", s.WindowCount())
	}
}

func TestPrefixSplitAndNavigate(t *testing.T) {
	s := newSession(t)
	d := New(s, 0)
	feed(d, []byte{DefaultPrefix, '%'}) // vertical split, right pane active (id 1)
	if len(s.ActiveWindow().Panes()) != 2 {
		t.Fatalf("prefix+%% should split")
	}
	if s.ActiveWindow().Layout.Active() != 1 {
		t.Fatalf("new pane should be active")
	}
	// prefix + Left arrow -> focus pane 0.
	feed(d, append([]byte{DefaultPrefix}, []byte("\x1b[D")...))
	if s.ActiveWindow().Layout.Active() != 0 {
		t.Fatalf("prefix+Left should focus pane 0, got %d", s.ActiveWindow().Layout.Active())
	}
}

func TestPrefixWindowCycle(t *testing.T) {
	s := newSession(t)
	d := New(s, 0)
	feed(d, []byte{DefaultPrefix, 'c'}) // window 1
	feed(d, []byte{DefaultPrefix, 'c'}) // window 2 active
	feed(d, []byte{DefaultPrefix, '0'}) // jump to window 0
	if s.ActiveIndex() != 0 {
		t.Fatalf("prefix+0 active = %d", s.ActiveIndex())
	}
	feed(d, []byte{DefaultPrefix, 'n'}) // next
	if s.ActiveIndex() != 1 {
		t.Fatalf("prefix+n active = %d", s.ActiveIndex())
	}
	feed(d, []byte{DefaultPrefix, 'p'}) // prev
	if s.ActiveIndex() != 0 {
		t.Fatalf("prefix+p active = %d", s.ActiveIndex())
	}
}

func TestPrefixCtrlArrowResizes(t *testing.T) {
	s := newSession(t)
	d := New(s, 0)
	feed(d, []byte{DefaultPrefix, '%'}) // vsplit, right pane active
	win := s.ActiveWindow()
	area := layout.Rect{X: 0, Y: 0, W: 80, H: 23}
	before := win.Layout.Geometry(area)[win.Layout.Active()].W
	// prefix + Ctrl+Left (ESC [ 1;5 D) grows the active pane.
	feed(d, append([]byte{DefaultPrefix}, []byte("\x1b[1;5D")...))
	after := win.Layout.Geometry(area)[win.Layout.Active()].W
	if after <= before {
		t.Fatalf("prefix+Ctrl+Left should resize: %d -> %d", before, after)
	}
}

func TestPrefixZoom(t *testing.T) {
	s := newSession(t)
	d := New(s, 0)
	feed(d, []byte{DefaultPrefix, '"'}) // split so zoom is meaningful
	feed(d, []byte{DefaultPrefix, 'z'})
	if !s.ActiveWindow().Layout.Zoomed() {
		t.Fatal("prefix+z should zoom")
	}
}

func TestCopyModeScroll(t *testing.T) {
	s := newSession(t)
	p := s.ActiveWindow().ActivePane()
	for i := 0; i < 40; i++ {
		p.Screen.Write([]byte("row\r\n"))
	}
	d := New(s, 0)
	feed(d, []byte{DefaultPrefix, '['}) // enter copy mode
	if !p.Copy {
		t.Fatal("prefix+[ should enter copy mode")
	}
	feed(d, []byte("\x1b[5~")) // PageUp
	if p.ScrollOff != 10 {
		t.Fatalf("PageUp should scroll 10, got %d", p.ScrollOff)
	}
	feed(d, []byte("\x1b[A")) // arrow up
	if p.ScrollOff != 11 {
		t.Fatalf("arrow up should scroll 1 more, got %d", p.ScrollOff)
	}
	feed(d, []byte{'q'}) // exit copy mode
	if p.Copy {
		t.Fatal("q should exit copy mode")
	}
}

func TestConfigurablePrefix(t *testing.T) {
	s := newSession(t)
	d := New(s, 0x01) // Ctrl+a
	// Ctrl+b should now be a literal keystroke, not the prefix.
	fwd, _ := feed(d, []byte{0x02})
	if !bytes.Equal(fwd, []byte{0x02}) {
		t.Fatalf("non-prefix control byte should forward, got %v", fwd)
	}
	feed(d, []byte{0x01, 'c'}) // Ctrl+a c -> new window
	if s.WindowCount() != 2 {
		t.Fatalf("custom prefix new-window failed, count=%d", s.WindowCount())
	}
}

func TestUnknownCommandIgnored(t *testing.T) {
	s := newSession(t)
	d := New(s, 0)
	_, res := feed(d, []byte{DefaultPrefix, '@'}) // no binding
	if res.Detach || res.Prompt || len(res.Forward) != 0 {
		t.Fatalf("unknown prefix command should be a no-op, got %+v", res)
	}
}

func TestPrefixColonOpensPrompt(t *testing.T) {
	d := New(newSession(t), 0)
	_, res := feed(d, []byte{DefaultPrefix, ':'})
	if !res.Prompt {
		t.Fatal("prefix+: should open the command prompt")
	}
}
