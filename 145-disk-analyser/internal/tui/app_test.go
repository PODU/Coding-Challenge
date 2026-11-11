package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ccdisk/internal/analysis"
	"ccdisk/internal/cleanup"
	"ccdisk/internal/scan"
	"ccdisk/internal/term"
	"ccdisk/internal/testtree"
	"ccdisk/internal/treemap"
)

const MB = testtree.MB

type dirTrash struct{ dir string }

func (d dirTrash) Where() string { return "the test trash" }
func (d dirTrash) Trash(p string) error {
	return os.Rename(p, filepath.Join(d.dir, filepath.Base(p)))
}

type harness struct {
	t      *testing.T
	a      *App
	c      *term.Canvas
	root   string
	trash  string
	window [2]int
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "testroot")
	if _, err := testtree.Build(root); err != nil {
		t.Fatal(err)
	}
	trash := filepath.Join(dir, "trash")
	os.Mkdir(trash, 0o755)
	a := New(root, scan.Options{}, dirTrash{trash})
	a.Sync = true
	a.StartScan(root)
	if a.Root == nil {
		t.Fatal(a.Status)
	}
	h := &harness{t: t, a: a, root: root, trash: trash}
	h.resize(140, 44)
	return h
}

func (h *harness) resize(w, ht int) { h.c = term.NewCanvas(w, ht); h.draw() }
func (h *harness) draw()            { h.a.Draw(h.c) }
func (h *harness) node(p string) *scan.Node {
	h.t.Helper()
	n := h.a.Root.Find(p)
	if n == nil {
		h.t.Fatalf("no node %s", p)
	}
	return n
}
func (h *harness) key(k term.Key) { h.a.Handle(term.Event{Key: k}); h.draw() }
func (h *harness) keys(s string) {
	for _, r := range s {
		h.a.Handle(term.Event{Key: term.KeyRune, Rune: r})
		h.draw()
	}
}
func (h *harness) click(x, y int) {
	h.a.Handle(term.Event{Key: term.KeyMouse, X: x, Y: y, Press: true})
	h.draw()
}
func (h *harness) screen() string { return h.c.String() }
func (h *harness) contains(s string) {
	h.t.Helper()
	if !strings.Contains(h.screen(), s) {
		h.t.Fatalf("screen does not contain %q:\n%s", s, h.screen())
	}
}
func (h *harness) box(p string) *treemap.Box {
	h.t.Helper()
	b := treemap.Find(h.a.Boxes(), h.node(p))
	if b == nil {
		h.t.Fatalf("%s not drawn", p)
	}
	return b
}

func TestInitialView(t *testing.T) {
	h := newHarness(t)
	if h.a.View != h.a.Root || h.a.Sel != h.node("media") {
		t.Fatal("should open on the root with the largest child selected")
	}
	h.contains("testroot")
	h.contains("size: disk usage")
	if !strings.Contains(h.a.Status, "Scanned") {
		t.Fatal(h.a.Status)
	}
	if b := h.box("media"); b.Kind != treemap.DirBox {
		t.Fatal("media should be drawn as a container")
	}
	if b := h.box("projects"); b.Kind != treemap.DirBox {
		t.Fatal("projects should be drawn as a container")
	}
	// The hard link must not be a second big rectangle.
	if treemap.Find(h.a.Boxes(), h.node("media/video-link.bin")) != nil {
		t.Fatal("video-link.bin should not be drawn")
	}
	v, p1 := h.box("media/video.bin"), h.box("media/photo1.bin")
	if r := float64(v.R.Area()) / float64(p1.R.Area()); r < 4 || r > 6.5 {
		t.Fatalf("video/photo1 area ratio %.1f, want about 5", r)
	}
}

func TestSelectionHighlight(t *testing.T) {
	h := newHarness(t)
	h.a.Select(h.node("media/video.bin"))
	h.draw()
	b := h.box("media/video.bin")
	if s := h.c.StyleAt(b.R.X+1, b.R.Y+1); s.Bg != 231 {
		t.Fatalf("selected file should be highlighted, style %+v", s)
	}
	h.contains("80.0% of " + filepath.Join("testroot", "media"))
}

func TestClickSelectsAndDoubleClickOpens(t *testing.T) {
	h := newHarness(t)
	b := h.box("media/video.bin")
	h.click(b.R.X+1, b.R.Y+1)
	if h.a.Sel != h.node("media/video.bin") {
		t.Fatal("click should select video.bin")
	}
	m := h.box("media")
	h.click(m.R.X+2, m.R.Y) // the frame's title row
	h.click(m.R.X+2, m.R.Y)
	if h.a.View != h.node("media") {
		t.Fatal("double click should open media")
	}
}

func TestOpenBackAndTop(t *testing.T) {
	h := newHarness(t)
	h.a.Select(h.node("media"))
	h.key(term.KeyEnter)
	if h.a.View != h.node("media") {
		t.Fatal("enter should open media")
	}
	var drawn []string
	for _, b := range h.a.Boxes() {
		drawn = append(drawn, b.Node.Name)
	}
	if len(drawn) != 3 {
		t.Fatalf("media should show its three files, got %v", drawn)
	}
	h.key(term.KeyBackspace)
	if h.a.View != h.a.Root || h.a.Sel != h.node("media") {
		t.Fatal("back should return to testroot with media selected")
	}
	h.a.Open(h.node("projects/webapp/node_modules"))
	h.draw()
	h.contains("node_modules")
	// Click the root crumb: back to the top in one action.
	row := h.c.Row(0)
	x := strings.Index(row, "testroot")
	h.click(term.TextWidth(row[:x]), 0)
	if h.a.View != h.a.Root {
		t.Fatal("clicking the root breadcrumb should jump to the top")
	}
	h.a.Open(h.node("projects/webapp/node_modules"))
	h.keys("t")
	if h.a.View != h.a.Root {
		t.Fatal("t should jump to the top")
	}
}

func TestArrowKeysMoveBetweenSiblings(t *testing.T) {
	h := newHarness(t)
	seen := map[*scan.Node]bool{h.a.Sel: true}
	for _, k := range []term.Key{term.KeyRight, term.KeyDown, term.KeyLeft, term.KeyUp, term.KeyRight, term.KeyRight, term.KeyDown} {
		h.key(k)
		seen[h.a.Sel] = true
		if h.a.Sel.Parent != h.a.Root {
			t.Fatalf("arrows should stay among testroot's children, got %s", h.a.Sel.Path())
		}
	}
	if len(seen) < 3 {
		t.Fatalf("arrows visited only %d items", len(seen))
	}
	h.keys("]")
	if h.a.Sel.Parent == h.a.Root {
		t.Fatal("] should select inside the directory")
	}
	h.keys("[")
	if h.a.Sel.Parent != h.a.Root {
		t.Fatal("[ should select the parent")
	}
}

func TestResizeRelayoutsWithoutOverlap(t *testing.T) {
	h := newHarness(t)
	for _, sz := range [][2]int{{80, 24}, {200, 60}, {41, 10}, {120, 30}} {
		h.resize(sz[0], sz[1])
		boxes := h.a.Boxes()
		r := h.a.mapRect
		if r.W != sz[0]-panelWidth(sz[0]) || r.H != sz[1]-3 {
			t.Fatalf("map rect %+v for %v", r, sz)
		}
		// Children of each parent tile its area exactly.
		parents := map[*scan.Node]treemap.Rect{h.a.View: r}
		for _, b := range boxes {
			if b.Kind == treemap.DirBox {
				parents[b.Node] = b.Inner
			}
		}
		for p, pr := range parents {
			grid := make([]int, pr.W*pr.H)
			any := false
			for _, b := range boxes {
				if b.Parent != p {
					continue
				}
				any = true
				for y := b.R.Y; y < b.R.Y+b.R.H; y++ {
					for x := b.R.X; x < b.R.X+b.R.W; x++ {
						if !pr.Contains(x, y) {
							t.Fatalf("%v: box outside its parent", sz)
						}
						grid[(y-pr.Y)*pr.W+x-pr.X]++
					}
				}
			}
			for i, n := range grid {
				if any && n != 1 {
					t.Fatalf("%v: %s cell %d covered %d times", sz, p.Name, i, n)
				}
			}
		}
	}
}

func panelWidth(w int) int {
	if w < 70 {
		return 0
	}
	return min(max(w/3, 34), 56)
}

func TestInspector(t *testing.T) {
	h := newHarness(t)
	h.a.Select(h.node("media"))
	h.draw()
	h.contains("Disk usage:    50.0 MB")
	h.contains("Files: 4")
	h.contains("Modified: ")
	if h.a.panelRows[0] != h.node("media/video.bin") {
		t.Fatal("video.bin should head the largest-children list")
	}
	h.keys("1")
	if h.a.Sel != h.node("media/video.bin") {
		t.Fatal("1 should jump to video.bin")
	}
	// The same through the panel's keyboard focus.
	h.a.Select(h.node("media"))
	h.key(term.KeyTab)
	h.key(term.KeyEnter)
	if h.a.Sel != h.node("media/video.bin") || !h.a.Focus {
		t.Fatal("enter in the focused panel should select the row")
	}
}

func TestAgeColours(t *testing.T) {
	h := newHarness(t)
	h.keys("g")
	h.contains("> 1 year")
	b := h.box("old/archive.bin")
	if s := h.c.StyleAt(b.R.X, b.R.Y); s.Bg != ageColours[analysis.Old].bg {
		t.Fatalf("archive.bin should be in the oldest band, style %+v", s)
	}
	p := h.box("media/photo1.bin")
	if s := h.c.StyleAt(p.R.X, p.R.Y); s.Bg != ageColours[analysis.Fresh].bg {
		t.Fatalf("new file should be in the newest band, style %+v", s)
	}
}

func TestLargestPanelAndFilters(t *testing.T) {
	h := newHarness(t)
	h.keys("l")
	want := []string{"media/video.bin", "projects/webapp/node_modules/lib/big.js", "old/archive.bin", ".cache/app/cache.db"}
	for i, w := range want {
		if h.a.panelRows[i] != h.node(w) {
			t.Fatalf("row %d is %s, want %s", i, h.a.panelRows[i].Path(), w)
		}
	}
	h.keys("mmoo") // ≥10 MB, ≥1 year
	if len(h.a.panelRows) != 1 || h.a.panelRows[0] != h.node("old/archive.bin") {
		t.Fatalf("filters should leave only archive.bin, got %d rows", len(h.a.panelRows))
	}
	h.keys("mmmo") // cycle both filters back to "any"
	h.a.Open(h.node("media"))
	h.draw()
	for _, n := range h.a.panelRows {
		if !n.Within(h.node("media")) {
			t.Fatal("after opening media the list should only show its files")
		}
	}
	h.key(term.KeyTab)
	h.key(term.KeyDown)
	h.key(term.KeyEnter)
	if h.a.Sel != h.a.panelRows[1] {
		t.Fatal("selecting a row selects the file in the treemap")
	}
}

func TestReclaimableMarked(t *testing.T) {
	h := newHarness(t)
	h.contains("♻ reclaimable here: 34.0 MB (3)")
	for _, p := range []string{"projects/webapp/node_modules", "projects/api/target", ".cache"} {
		if _, ok := h.a.reclaimable(h.node(p)); !ok {
			t.Fatalf("%s should be marked", p)
		}
	}
	for _, p := range []string{"docs", "media"} {
		if _, ok := h.a.reclaimable(h.node(p)); ok {
			t.Fatalf("%s should not be marked", p)
		}
	}
}

func TestStagingAndReviewTouchNothing(t *testing.T) {
	h := newHarness(t)
	before := countFiles(h.root)
	h.a.Select(h.node("old/archive.bin"))
	h.keys(" ")
	h.contains("staged: 15.0 MB (1)")
	h.a.Select(h.node("projects/webapp/node_modules"))
	h.keys(" ")
	h.contains("staged: 36.0 MB (2)")
	h.a.Select(h.node("projects/webapp/node_modules/lib/big.js"))
	h.keys(" ")
	h.contains("staged: 36.0 MB (3)")
	free := h.a.Free
	h.contains("free: " + analysis.HumanSize(free) + " → " + analysis.HumanSize(free+h.a.Staging.Total(scan.DiskUsage)))
	h.keys(" ") // unstage big.js
	h.contains("staged: 36.0 MB (2)")

	h.keys("c")
	if h.a.Mode != ModeReview || len(h.a.review) != 2 {
		t.Fatal("c should open the review with both items")
	}
	h.contains("Review: 2 item(s)")
	h.keys("d") // drop the first (node_modules sorts before old)
	if len(h.a.review) != 1 || h.a.Staging.Len() != 1 {
		t.Fatal("dropping should update the list and the staged set")
	}
	h.contains("Review: 1 item(s)")
	h.key(term.KeyEsc)
	if h.a.Mode != ModeBrowse || countFiles(h.root) != before {
		t.Fatal("cancelling must leave every file in place")
	}
}

func TestStagingRefusals(t *testing.T) {
	h := newHarness(t)
	h.a.Staging.Policy = analysis.DefaultPolicy(h.node("media").Path())
	h.a.Select(h.node("docs/report.pdf"))
	h.keys(" ")
	if h.a.Staging.Len() != 0 || !strings.Contains(h.a.Status, "outside the scanned root") {
		t.Fatalf("staging outside the root should be refused with a reason: %q", h.a.Status)
	}
}

func TestCleanupMovesAndRescans(t *testing.T) {
	h := newHarness(t)
	total := h.a.Root.TotalUsage
	for _, p := range []string{"old/archive.bin", ".cache/app/cache.db"} {
		h.a.Select(h.node(p))
		h.keys(" ")
	}
	h.keys("c")
	h.keys("y")
	if h.a.Mode != ModeReview || !h.a.confirm {
		t.Fatal("first y should ask for confirmation")
	}
	h.contains("Press y to confirm")
	h.keys("y")
	if h.a.Mode != ModeResults {
		t.Fatalf("mode %v", h.a.Mode)
	}
	r := h.a.Report
	if r.Count(cleanup.Moved) != 2 || r.Moved < 25*MB || r.Moved > 25*MB+64<<10 {
		t.Fatalf("report %+v", r)
	}
	h.contains("Moved to the test trash: 2 item(s), 25.0 MB")
	if h.a.Root.Find("old/archive.bin") != nil || h.a.Root.Find(".cache/app/cache.db") != nil {
		t.Fatal("moved items should be gone from the tree after the rescan")
	}
	if got := total - h.a.Root.TotalUsage; got < 25*MB || got > 25*MB+64<<10 {
		t.Fatalf("total shrank by %d, want 25 MB", got)
	}
	if _, err := os.Stat(filepath.Join(h.trash, "archive.bin")); err != nil {
		t.Fatal("archive.bin should be in the trash")
	}
	if h.a.Staging.Len() != 0 {
		t.Fatal("moved items should leave the staged set")
	}
	h.key(term.KeyEnter)
	if h.a.Mode != ModeBrowse {
		t.Fatal("enter should return to the treemap")
	}
}

func TestCleanupSkipsChangedItems(t *testing.T) {
	h := newHarness(t)
	for _, p := range []string{"old/archive.bin", "docs/notes.txt"} {
		h.a.Select(h.node(p))
		h.keys(" ")
	}
	h.keys("c")
	// From "another terminal" while the review is open.
	os.Remove(filepath.Join(h.root, "old", "archive.bin"))
	os.Remove(filepath.Join(h.root, "docs", "notes.txt"))
	os.Mkdir(filepath.Join(h.root, "docs", "notes.txt"), 0o755)
	h.keys("yy")
	r := h.a.Report
	if r.Count(cleanup.Skipped) != 2 || r.Count(cleanup.Moved) != 0 {
		t.Fatalf("both should be skipped: %+v", r.Outcomes)
	}
	h.contains("no longer exists")
	h.contains("changed from a file to a directory")
	if fi, err := os.Stat(filepath.Join(h.root, "docs", "notes.txt")); err != nil || !fi.IsDir() {
		t.Fatal("the new directory must be untouched")
	}
	if entries, _ := os.ReadDir(h.trash); len(entries) != 0 {
		t.Fatal("nothing should have been moved")
	}
}

func TestRescanCurrentFolder(t *testing.T) {
	h := newHarness(t)
	h.a.Open(h.node("docs"))
	os.WriteFile(filepath.Join(h.root, "docs", "new.txt"), make([]byte, 3*MB), 0o644)
	h.keys("r")
	if h.a.Root.Find("docs/new.txt") == nil || h.a.View != h.node("docs") {
		t.Fatal("r should rescan the open folder in place")
	}
	if h.node("docs").TotalUsage < 8*MB {
		t.Fatal("totals should include the new file")
	}
}

func TestWarningsPanelAndBusyScreen(t *testing.T) {
	h := newHarness(t)
	h.a.Warnings = []scan.Warning{{Path: filepath.Join(h.root, "locked"), Err: os.ErrPermission}}
	h.keys("w")
	h.contains("could not be read")
	h.contains("locked")
	h.a.Busy, h.a.Progress = "Scanning x", &scan.Progress{}
	h.a.Progress.Items.Store(1234)
	h.draw()
	h.contains("1234 items")
	h.a.Handle(term.Event{Key: term.KeyRune, Rune: 'a'})
	if h.a.Measure != scan.DiskUsage {
		t.Fatal("keys other than q are ignored while busy")
	}
	h.a.Handle(term.Event{Key: term.KeyRune, Rune: 'q'})
	if !h.a.Quit {
		t.Fatal("q should stop while busy")
	}
}

func TestApparentToggleLabels(t *testing.T) {
	h := newHarness(t)
	h.keys("a")
	h.contains("size: apparent size")
	if h.box("sparse.img") == nil {
		t.Fatal("by apparent size the sparse file dominates")
	}
}

func TestTinyTerminal(t *testing.T) {
	h := newHarness(t)
	h.resize(30, 8)
	h.contains("Terminal too small")
}

func countFiles(root string) int {
	n := 0
	filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			n++
		}
		return nil
	})
	return n
}
