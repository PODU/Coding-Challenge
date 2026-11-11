// Package tui is the interactive treemap browser. App holds all state and
// reacts to events; Draw paints it onto a canvas. Neither does terminal I/O,
// so the whole UI is testable with plain function calls.
package tui

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"ccdisk/internal/analysis"
	"ccdisk/internal/cleanup"
	"ccdisk/internal/scan"
	"ccdisk/internal/term"
	"ccdisk/internal/treemap"
)

type Panel uint8

const (
	PanelInfo Panel = iota
	PanelLargest
	PanelWarnings
)

type ColorMode uint8

const (
	ColorType ColorMode = iota
	ColorAge
)

type Mode uint8

const (
	ModeBrowse Mode = iota
	ModeReview
	ModeResults
)

var (
	minSizes = []int64{0, 1 << 20, 10 << 20, 100 << 20, 1 << 30}
	minAges  = []time.Duration{0, analysis.Month, analysis.Year}
	ageNames = []string{"any age", "≥1 month old", "≥1 year old"}
)

type hit struct {
	x0, x1, y int
	act       func(double bool)
}

type App struct {
	Root     *scan.Node
	View     *scan.Node // the directory filling the treemap
	Sel      *scan.Node
	Measure  scan.Measure
	Colors   ColorMode
	Panel    Panel
	Focus    bool // keyboard drives the side panel instead of the treemap
	PanelIdx int
	MinSize  int // index into minSizes
	MinAge   int // index into minAges
	Staging  *analysis.Staging
	Warnings []scan.Warning
	Trasher  cleanup.Trasher
	ScanOpts scan.Options
	Now      func() time.Time
	Sync     bool // run background jobs inline (tests)

	Mode      Mode
	review    []*scan.Node
	reviewIdx int
	confirm   bool
	Report    *cleanup.Report

	Free, Total int64
	FreeErr     error

	Status string
	Quit   bool

	// Background work: the label shown while it runs, its progress, and
	// the channel its result arrives on.
	Busy     string
	Progress *scan.Progress
	Done     chan func()
	cancel   context.CancelFunc
	started  time.Time

	// Rebuilt by Draw.
	boxes      []treemap.Box
	mapRect    treemap.Rect
	layoutKey  layoutKey
	hits       []hit
	panelRows  []*scan.Node
	version    int
	cache      cacheEntry
	lastClick  time.Time
	lastTarget *scan.Node
}

type layoutKey struct {
	view    *scan.Node
	measure scan.Measure
	r       treemap.Rect
	version int
}

type cacheEntry struct {
	view       *scan.Node
	version    int
	measure    scan.Measure
	minSize    int
	minAge     int
	reclaim    []analysis.Candidate
	reclaimSet map[*scan.Node]analysis.Category
	largest    []*scan.Node
}

func New(root string, opts scan.Options, trasher cleanup.Trasher) *App {
	return &App{
		ScanOpts: opts,
		Trasher:  trasher,
		Staging:  analysis.NewStaging(analysis.DefaultPolicy(root)),
		Now:      time.Now,
		Done:     make(chan func(), 4),
	}
}

func (a *App) start(label string, job func(ctx context.Context) func()) {
	ctx, cancel := context.WithCancel(context.Background())
	a.Busy, a.cancel, a.started = label, cancel, a.Now()
	if a.Progress == nil {
		a.Progress = &scan.Progress{}
	}
	if a.Sync {
		apply := job(ctx)
		a.Busy = ""
		apply()
		return
	}
	go func() {
		apply := job(ctx)
		a.Done <- func() { a.Busy = ""; apply() }
	}()
}

// StartScan scans root in the background.
func (a *App) StartScan(root string) {
	a.Progress = &scan.Progress{}
	opts := a.ScanOpts
	opts.Progress = a.Progress
	a.start("Scanning "+root, func(ctx context.Context) func() {
		t0 := time.Now()
		n, warns, err := scan.Scan(ctx, root, opts)
		elapsed := time.Since(t0)
		return func() {
			if err != nil && n == nil {
				a.Status = "scan failed: " + err.Error()
				a.Quit = true
				return
			}
			a.Root, a.View, a.Warnings = n, n, warns
			a.Staging.Policy = analysis.DefaultPolicy(n.Path())
			a.selectDefault()
			a.refreshFree()
			a.changed()
			a.Status = fmt.Sprintf("Scanned %d items, %s, in %s", n.Items+1, analysis.HumanSize(n.TotalUsage), elapsed.Round(time.Millisecond))
			if len(warns) > 0 {
				a.Status += fmt.Sprintf(" · %d warning(s), press w to list: %s", len(warns), warns[0])
			}
			if err != nil {
				a.Status = "scan stopped early: " + err.Error()
			}
		}
	})
}

func (a *App) rescanView() {
	dir := a.View
	a.Progress = &scan.Progress{}
	opts := a.ScanOpts
	opts.Progress = a.Progress
	a.start("Rescanning "+dir.DisplayPath(), func(ctx context.Context) func() {
		warns, err := scan.Rescan(ctx, dir, opts)
		return func() {
			a.afterRescan()
			a.Status = "Rescanned " + dir.DisplayPath()
			if err != nil {
				a.Status = "rescan: " + err.Error()
			} else if len(warns) > 0 {
				a.Warnings = append(a.Warnings, warns...)
				a.Status += fmt.Sprintf(" · %d warning(s)", len(warns))
			}
		}
	})
}

func (a *App) afterRescan() {
	scan.Recompute(a.Root)
	a.Staging.Forget(a.Root)
	if !attached(a.View) {
		a.View = a.Root
	}
	if a.Sel == nil || !attached(a.Sel) || !a.Sel.Within(a.View) || a.Sel == a.View {
		a.selectDefault()
	}
	a.refreshFree()
	a.changed()
}

func attached(n *scan.Node) bool {
	for ; n.Parent != nil; n = n.Parent {
		found := false
		for _, c := range n.Parent.Children {
			if c == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (a *App) refreshFree() {
	if a.Root != nil {
		a.Free, a.Total, a.FreeErr = scan.FreeSpace(a.Root.Path())
	}
}

// changed invalidates everything derived from the tree.
func (a *App) changed() { a.version++ }

func (a *App) derived() *cacheEntry {
	c := &a.cache
	if c.view == a.View && c.version == a.version && c.measure == a.Measure && c.minSize == a.MinSize && c.minAge == a.MinAge && c.reclaimSet != nil {
		return c
	}
	*c = cacheEntry{view: a.View, version: a.version, measure: a.Measure, minSize: a.MinSize, minAge: a.MinAge}
	c.reclaim = analysis.Reclaimable(a.View)
	c.reclaimSet = map[*scan.Node]analysis.Category{}
	for _, r := range c.reclaim {
		c.reclaimSet[r.Node] = r.Category
	}
	// A view opened inside a reclaimable directory is itself marked.
	if cat, ok := analysis.Classify(a.View); ok {
		c.reclaimSet[a.View] = cat
	}
	c.largest = analysis.Largest(a.View, analysis.Filter{
		MinSize: minSizes[a.MinSize], MinAge: minAges[a.MinAge], Measure: a.Measure, Now: a.Now(),
	}, 200)
	return c
}

func (a *App) reclaimable(n *scan.Node) (analysis.Category, bool) {
	cat, ok := a.derived().reclaimSet[n]
	return cat, ok
}

func (a *App) selectDefault() {
	kids := analysis.LargestChildren(a.View, a.Measure)
	if len(kids) > 0 {
		a.Sel = kids[0]
	} else {
		a.Sel = nil
	}
}

// Open makes dir fill the treemap.
func (a *App) Open(dir *scan.Node) {
	if dir == nil || dir.Kind != scan.Dir {
		return
	}
	prev := a.View
	a.View = dir
	a.PanelIdx = 0
	if prev != nil && prev.Within(dir) && prev != dir {
		// Going up: keep the way we came selected.
		for p := prev; p != nil; p = p.Parent {
			if p.Parent == dir {
				a.Sel = p
				return
			}
		}
	}
	if a.Sel == nil || !a.Sel.Within(dir) || a.Sel == dir {
		a.selectDefault()
	}
}

func (a *App) Back() {
	if a.View.Parent != nil {
		a.Open(a.View.Parent)
	}
}

func (a *App) Top() { a.Open(a.Root) }

// Select moves the selection, opening a parent directory if the node is
// outside the current view.
func (a *App) Select(n *scan.Node) {
	if n == nil {
		return
	}
	if !n.Within(a.View) || n == a.View {
		if n.Parent != nil {
			a.Open(n.Parent)
		}
	}
	a.Sel = n
}

// Move steps the selection to the nearest sibling in a direction, using
// the rectangles on screen. With no sibling that way it tries the parent's
// siblings, so the selection can climb out of a nested directory.
func (a *App) Move(dx, dy int) {
	if a.Sel == nil {
		a.selectDefault()
		return
	}
	for n := a.Sel; n != nil && n != a.View; n = n.Parent {
		from := treemap.Find(a.boxes, n)
		if from == nil {
			continue
		}
		cx, cy := from.R.Center()
		cy *= 2 // cells are about twice as tall as wide
		best, bestScore := (*scan.Node)(nil), math.Inf(1)
		for i := range a.boxes {
			b := &a.boxes[i]
			if b.Node == nil || b.Node == n || b.Node.Parent != n.Parent {
				continue
			}
			bx, by := b.R.Center()
			by *= 2
			vx, vy := bx-cx, by-cy
			along := vx*float64(dx) + vy*float64(dy)
			if along <= 0.5 {
				continue
			}
			perp := math.Abs(vx*float64(dy)) + math.Abs(vy*float64(dx))
			if score := along + 2*perp; score < bestScore {
				best, bestScore = b.Node, score
			}
		}
		if best != nil {
			a.Sel = best
			return
		}
	}
}

func (a *App) descend() {
	if a.Sel != nil && a.Sel.Kind == scan.Dir {
		if kids := analysis.LargestChildren(a.Sel, a.Measure); len(kids) > 0 {
			a.Sel = kids[0]
		}
	}
}

func (a *App) ascend() {
	if a.Sel != nil && a.Sel.Parent != nil && a.Sel.Parent != a.View && a.Sel.Parent.Within(a.View) {
		a.Sel = a.Sel.Parent
	}
}

func (a *App) toggleStage(n *scan.Node) {
	if n == nil {
		return
	}
	on, err := a.Staging.Toggle(n)
	switch {
	case err != nil:
		reason, _ := analysis.IsRefused(err)
		a.Status = "Cannot stage " + n.DisplayPath() + ": " + reason
	case on:
		a.Status = "Staged " + n.DisplayPath() + " (nothing is removed until you review and confirm)"
	default:
		a.Status = "Unstaged " + n.DisplayPath()
	}
}

func (a *App) openReview() {
	if a.Staging.Len() == 0 {
		a.Status = "Nothing staged. Select an item and press space to stage it."
		return
	}
	a.Mode, a.reviewIdx, a.confirm = ModeReview, 0, false
	a.review = a.Staging.Items()
}

func (a *App) dropFromReview() {
	if a.reviewIdx < len(a.review) {
		a.Staging.Remove(a.review[a.reviewIdx])
		a.review = a.Staging.Items()
		if a.reviewIdx >= len(a.review) {
			a.reviewIdx = max(0, len(a.review)-1)
		}
		if len(a.review) == 0 {
			a.Mode = ModeBrowse
			a.Status = "Review list is empty; nothing was removed."
		}
	}
}

func (a *App) confirmCleanup() {
	items := append([]*scan.Node(nil), a.review...)
	pol := a.Staging.Policy
	trasher := a.Trasher
	a.Progress = nil
	a.start("Moving "+fmt.Sprint(len(items))+" item(s) to "+trasher.Where(), func(context.Context) func() {
		rep := cleanup.Execute(items, pol, trasher)
		// Rescan every directory that held a reviewed item, so totals come
		// from what the filesystem now says, not from the estimate.
		dirs := affectedDirs(items)
		for _, d := range dirs {
			scan.Rescan(context.Background(), d, a.ScanOpts)
		}
		return func() {
			for _, o := range rep.Outcomes {
				if o.Status == cleanup.Moved {
					a.Staging.Remove(o.Node)
				}
			}
			a.afterRescan()
			a.Report = &rep
			a.Mode = ModeResults
		}
	})
}

// affectedDirs is the parents of items, without any that lie inside another.
func affectedDirs(items []*scan.Node) []*scan.Node {
	set := map[*scan.Node]bool{}
	for _, n := range items {
		if n.Parent != nil {
			set[n.Parent] = true
		}
	}
	var out []*scan.Node
	for d := range set {
		inner := false
		for p := d.Parent; p != nil; p = p.Parent {
			if set[p] {
				inner = true
				break
			}
		}
		if !inner {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path() < out[j].Path() })
	return out
}

func (a *App) Handle(ev term.Event) {
	if ev.Key == term.KeyCtrlC {
		a.quit()
		return
	}
	if a.Busy != "" {
		if ev.Key == term.KeyEsc || (ev.Key == term.KeyRune && ev.Rune == 'q') {
			a.quit()
		}
		return
	}
	if ev.Key != term.KeyMouse {
		a.Status = ""
	}
	switch a.Mode {
	case ModeReview:
		a.handleReview(ev)
	case ModeResults:
		if ev.Key == term.KeyEsc || ev.Key == term.KeyEnter || (ev.Key == term.KeyRune && ev.Rune == 'q') {
			a.Mode = ModeBrowse
		}
	default:
		a.handleBrowse(ev)
	}
}

func (a *App) quit() {
	if a.cancel != nil {
		a.cancel()
	}
	a.Quit = true
}

func (a *App) handleReview(ev term.Event) {
	if a.confirm {
		a.confirm = false
		if ev.Key == term.KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y') {
			a.confirmCleanup()
		} else {
			a.Status = "Cancelled; nothing was removed."
		}
		return
	}
	switch {
	case ev.Key == term.KeyUp:
		a.reviewIdx = max(0, a.reviewIdx-1)
	case ev.Key == term.KeyDown:
		a.reviewIdx = min(len(a.review)-1, a.reviewIdx+1)
	case ev.Key == term.KeyDelete, ev.Key == term.KeyBackspace, ev.Key == term.KeyRune && (ev.Rune == 'd' || ev.Rune == ' '):
		a.dropFromReview()
	case ev.Key == term.KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y'):
		a.confirm = true
	case ev.Key == term.KeyEsc, ev.Key == term.KeyRune && (ev.Rune == 'q' || ev.Rune == 'n'):
		a.Mode = ModeBrowse
		a.Status = "Review closed; nothing was removed."
	case ev.Key == term.KeyMouse && ev.Press && ev.Button == 0:
		a.click(ev.X, ev.Y)
	}
}

func (a *App) handleBrowse(ev term.Event) {
	if a.Root == nil {
		if ev.Key == term.KeyRune && ev.Rune == 'q' {
			a.quit()
		}
		return
	}
	if ev.Key == term.KeyMouse {
		if ev.Press && ev.Button == 0 {
			a.click(ev.X, ev.Y)
		}
		return
	}
	if a.Focus && a.handlePanelKey(ev) {
		return
	}
	switch ev.Key {
	case term.KeyUp:
		a.Move(0, -1)
	case term.KeyDown:
		a.Move(0, 1)
	case term.KeyLeft:
		a.Move(-1, 0)
	case term.KeyRight:
		a.Move(1, 0)
	case term.KeyEnter:
		if a.Sel != nil && a.Sel.Kind == scan.Dir {
			a.Open(a.Sel)
		} else if a.Sel != nil {
			a.Status = a.Sel.Name + " is not a directory"
		}
	case term.KeyBackspace:
		a.Back()
	case term.KeyHome:
		a.Top()
	case term.KeyTab, term.KeyBacktab:
		a.Focus = !a.Focus
		a.PanelIdx = 0
	case term.KeyF5:
		a.rescanView()
	case term.KeyEsc:
		a.Focus = false
	case term.KeyRune:
		a.handleRune(ev.Rune)
	}
}

func (a *App) handleRune(r rune) {
	switch r {
	case 'q':
		a.quit()
	case 'u':
		a.Back()
	case 't':
		a.Top()
	case ']':
		a.descend()
	case '[':
		a.ascend()
	case ' ', 's':
		a.toggleStage(a.Sel)
	case 'c':
		a.openReview()
	case 'a':
		if a.Measure == scan.DiskUsage {
			a.Measure = scan.ApparentSize
		} else {
			a.Measure = scan.DiskUsage
		}
		a.Status = "Sizes now show " + a.Measure.String()
	case 'g':
		if a.Colors == ColorType {
			a.Colors = ColorAge
		} else {
			a.Colors = ColorType
		}
	case 'i':
		a.Panel, a.PanelIdx = PanelInfo, 0
	case 'l':
		a.Panel, a.PanelIdx = PanelLargest, 0
	case 'w':
		a.Panel, a.PanelIdx = PanelWarnings, 0
	case 'm':
		a.MinSize = (a.MinSize + 1) % len(minSizes)
		a.Panel, a.PanelIdx = PanelLargest, 0
	case 'o':
		a.MinAge = (a.MinAge + 1) % len(minAges)
		a.Panel, a.PanelIdx = PanelLargest, 0
	case 'r':
		a.rescanView()
	default:
		if r >= '1' && r <= '9' {
			i := int(r - '1')
			if a.Panel == PanelInfo && i < len(a.panelRows) {
				a.Select(a.panelRows[i])
			}
		}
	}
}

func (a *App) handlePanelKey(ev term.Event) bool {
	switch ev.Key {
	case term.KeyUp:
		a.PanelIdx = max(0, a.PanelIdx-1)
	case term.KeyDown:
		a.PanelIdx = min(max(0, len(a.panelRows)-1), a.PanelIdx+1)
	case term.KeyEnter:
		if a.PanelIdx < len(a.panelRows) {
			a.Select(a.panelRows[a.PanelIdx])
		}
	default:
		return false
	}
	return true
}

func (a *App) click(x, y int) {
	now := a.Now()
	for _, h := range a.hits {
		if y == h.y && x >= h.x0 && x < h.x1 {
			h.act(false)
			return
		}
	}
	if a.Mode != ModeBrowse {
		return
	}
	b := treemap.At(a.boxes, x, y)
	if b == nil {
		return
	}
	var target *scan.Node
	if b.Kind == treemap.GroupBox {
		var size int64
		for _, m := range b.Group {
			size += m.Size(a.Measure)
		}
		a.Status = fmt.Sprintf("%d smaller items in %s, %s: open the folder or see the Info list",
			len(b.Group), b.Parent.DisplayPath(), analysis.HumanSize(size))
		if b.Parent != a.View {
			target = b.Parent
		}
	} else {
		target = b.Node
	}
	if target == nil {
		return
	}
	double := target == a.lastTarget && now.Sub(a.lastClick) < 400*time.Millisecond
	a.lastClick, a.lastTarget = now, target
	a.Sel = target
	if double && target.Kind == scan.Dir {
		a.Open(target)
	}
}
