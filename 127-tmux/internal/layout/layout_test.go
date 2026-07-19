package layout

import "testing"

func TestSinglePaneFillsWindow(t *testing.T) {
	tr := New()
	geo := tr.Geometry(Rect{0, 0, 80, 24})
	if len(geo) != 1 {
		t.Fatalf("want 1 pane, got %d", len(geo))
	}
	if geo[0] != (Rect{0, 0, 80, 24}) {
		t.Fatalf("pane 0 = %+v", geo[0])
	}
}

func TestHorizontalSplitTopBottom(t *testing.T) {
	tr := New()
	id := tr.Split(Horizontal) // pane 1 below pane 0
	if id != 1 {
		t.Fatalf("new id = %d", id)
	}
	geo := tr.Geometry(Rect{0, 0, 80, 25})
	// 25 rows: 1 divider row => 24 usable, split 12/12.
	top, bot := geo[0], geo[1]
	if top.H+bot.H != 24 {
		t.Fatalf("heights %d+%d != 24", top.H, bot.H)
	}
	if top.Y != 0 || bot.Y != top.H+1 {
		t.Fatalf("stacking wrong: top=%+v bot=%+v", top, bot)
	}
	if top.W != 80 || bot.W != 80 {
		t.Fatalf("widths should be full: %+v %+v", top, bot)
	}
	if tr.Active() != 1 {
		t.Fatalf("new pane should be active")
	}
}

func TestVerticalSplitLeftRight(t *testing.T) {
	tr := New()
	tr.Split(Vertical)
	geo := tr.Geometry(Rect{0, 0, 81, 24})
	// 81 cols: 1 divider col => 80 usable, 40/40.
	l, r := geo[0], geo[1]
	if l.W+r.W != 80 {
		t.Fatalf("widths %d+%d != 80", l.W, r.W)
	}
	if l.X != 0 || r.X != l.W+1 {
		t.Fatalf("side-by-side wrong: l=%+v r=%+v", l, r)
	}
}

func TestBordersReported(t *testing.T) {
	tr := New()
	tr.Split(Vertical)
	bs := tr.Borders(Rect{0, 0, 81, 24})
	if len(bs) != 1 || !bs[0].Vertical {
		t.Fatalf("want one vertical border, got %+v", bs)
	}
	if bs[0].Rect.W != 1 || bs[0].Rect.H != 24 {
		t.Fatalf("vertical border geom %+v", bs[0].Rect)
	}
}

func TestGridLayoutComposesSplits(t *testing.T) {
	tr := New()
	tr.Split(Vertical)   // 0 | 1, active 1
	tr.Split(Horizontal) // right column becomes 1 over 2
	if tr.Count() != 3 {
		t.Fatalf("want 3 panes, got %d", tr.Count())
	}
	geo := tr.Geometry(Rect{0, 0, 81, 25})
	// pane 0 spans full height on the left; 1 and 2 stack on the right.
	if geo[0].H != 25 {
		t.Fatalf("left pane should span full height, got %d", geo[0].H)
	}
	if geo[1].X != geo[2].X {
		t.Fatalf("right panes should share X: %d vs %d", geo[1].X, geo[2].X)
	}
}

func TestNavigateDirectional(t *testing.T) {
	tr := New()
	tr.Split(Vertical) // 0 left, 1 right (active)
	win := Rect{0, 0, 81, 24}
	if tr.Navigate(win, Left) != 0 {
		t.Fatalf("left should reach pane 0")
	}
	if tr.Navigate(win, Right) != 1 {
		t.Fatalf("right should reach pane 1")
	}
	// No pane further left of 0.
	tr.SetActive(0)
	if tr.Navigate(win, Left) != 0 {
		t.Fatalf("navigating off-edge should stay put")
	}
}

func TestCloseCollapsesSibling(t *testing.T) {
	tr := New()
	tr.Split(Vertical) // panes 0,1
	if !tr.Close(1) {
		t.Fatalf("close should succeed with >1 pane")
	}
	if tr.Count() != 1 {
		t.Fatalf("want 1 pane after close, got %d", tr.Count())
	}
	geo := tr.Geometry(Rect{0, 0, 80, 24})
	if geo[0] != (Rect{0, 0, 80, 24}) {
		t.Fatalf("survivor should fill window, got %+v", geo[0])
	}
	// Closing the last pane returns false.
	if tr.Close(0) {
		t.Fatalf("closing last pane should return false")
	}
}

func TestResizeAdjustsRatio(t *testing.T) {
	tr := New()
	tr.Split(Vertical) // 0 | 1, active 1
	win := Rect{0, 0, 81, 24}
	before := tr.Geometry(win)[1].W
	tr.SetActive(1)
	if !tr.Resize(win, Left, 5) { // grow pane 1's left border leftward
		t.Fatalf("resize should succeed")
	}
	after := tr.Geometry(win)[1].W
	if after <= before {
		t.Fatalf("pane 1 should have grown: %d -> %d", before, after)
	}
}

func TestZoomIsolatesActive(t *testing.T) {
	tr := New()
	tr.Split(Vertical)
	tr.ToggleZoom()
	geo := tr.Geometry(Rect{0, 0, 81, 24})
	if len(geo) != 1 {
		t.Fatalf("zoom should show only active pane, got %d", len(geo))
	}
	if geo[tr.Active()] != (Rect{0, 0, 81, 24}) {
		t.Fatalf("zoomed pane should fill window")
	}
	if len(tr.Borders(Rect{0, 0, 81, 24})) != 0 {
		t.Fatalf("zoom should hide borders")
	}
}
