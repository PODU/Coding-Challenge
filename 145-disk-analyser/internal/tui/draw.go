package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"ccdisk/internal/analysis"
	"ccdisk/internal/cleanup"
	"ccdisk/internal/scan"
	"ccdisk/internal/term"
	"ccdisk/internal/treemap"
)

func st(fg, bg int16) term.Style { return term.Style{Fg: fg, Bg: bg} }

var (
	sNormal   = term.Default
	sDim      = st(244, -1)
	sBold     = term.Style{Fg: -1, Bg: -1, Bold: true}
	sHeader   = st(231, 24)
	sCrumb    = term.Style{Fg: 231, Bg: 24, Underline: true}
	sKey      = st(229, -1)
	sWarn     = st(214, -1)
	sBad      = st(203, -1)
	sGood     = st(114, -1)
	sSelRow   = st(16, 229)
	sFrame    = st(244, -1)
	sReclaim  = st(213, -1)
	sStaged   = st(203, -1)
	sSelFrame = term.Style{Fg: 226, Bg: -1, Bold: true}
	sGroup    = st(242, 235)
)

// Category colours for files: background and a readable foreground.
type colour struct{ bg, fg int16 }

var (
	catMedia   = colour{33, 16}
	catArchive = colour{133, 231}
	catDoc     = colour{37, 16}
	catCode    = colour{71, 16}
	catData    = colour{172, 16}
	catBinary  = colour{167, 231}
	catOther   = colour{246, 16}
	catLink    = colour{60, 231}
	ageColours = []colour{{34, 16}, {178, 16}, {160, 231}}
)

var extCategory = map[string]colour{}

func init() {
	for cat, exts := range map[colour]string{
		catMedia:   "jpg jpeg png gif bmp webp heic tiff svg mp4 mkv mov avi webm mp3 wav flac aac ogg m4a",
		catArchive: "zip tar gz tgz bz2 xz 7z rar zst img iso dmg vhd vhdx vmdk qcow2",
		catDoc:     "pdf doc docx xls xlsx ppt pptx odt txt md rtf epub csv",
		catCode:    "go js ts tsx jsx py rs c h cpp java kt rb php cs swift html css json yaml yml toml sh",
		catData:    "db sqlite sqlite3 parquet log dat pack idx",
		catBinary:  "exe dll so dylib o a lib bin class jar wasm pyc",
	} {
		for _, e := range strings.Fields(exts) {
			extCategory["."+e] = cat
		}
	}
}

var typeLegend = []struct {
	name string
	c    colour
}{{"media", catMedia}, {"archive", catArchive}, {"docs", catDoc}, {"code", catCode}, {"data", catData}, {"binary", catBinary}, {"other", catOther}}

func (a *App) fileColour(n *scan.Node) colour {
	if a.Colors == ColorAge {
		return ageColours[analysis.BandOf(analysis.AgeOf(n), a.Now())]
	}
	switch n.Kind {
	case scan.Symlink:
		return catLink
	case scan.Dir:
		return colour{238, 250}
	}
	if c, ok := extCategory[strings.ToLower(filepath.Ext(n.Name))]; ok {
		return c
	}
	return catOther
}

// Draw paints the whole UI.
func (a *App) Draw(c *term.Canvas) {
	c.Clear()
	a.hits = a.hits[:0]
	if c.W < 40 || c.H < 10 {
		c.Text(0, 0, "Terminal too small for ccdisk", sWarn, c.W)
		return
	}
	if a.Busy != "" {
		a.drawBusy(c)
		return
	}
	if a.Root == nil {
		c.Text(1, 1, a.Status, sBad, c.W-2)
		return
	}
	switch a.Mode {
	case ModeReview:
		a.drawReview(c)
		return
	case ModeResults:
		a.drawResults(c)
		return
	}
	panelW := 0
	if c.W >= 70 {
		panelW = min(max(c.W/3, 34), 56)
	}
	a.drawHeader(c)
	r := treemap.Rect{X: 0, Y: 1, W: c.W - panelW, H: c.H - 3}
	a.layout(r)
	a.drawMap(c)
	if panelW > 0 {
		a.drawPanel(c, c.W-panelW, 1, panelW, c.H-3)
	}
	a.drawTotals(c, c.H-2)
	a.drawStatus(c, c.H-1)
}

func (a *App) layout(r treemap.Rect) {
	key := layoutKey{a.View, a.Measure, r, a.version}
	if key == a.layoutKey && a.boxes != nil {
		return
	}
	a.layoutKey, a.mapRect = key, r
	a.boxes = treemap.Layout(a.View, r, treemap.Options{Measure: a.Measure})
}

// Boxes exposes the current layout, for tests.
func (a *App) Boxes() []treemap.Box { return a.boxes }

func (a *App) drawHeader(c *term.Canvas) {
	c.Fill(0, 0, c.W, 1, ' ', sHeader)
	right := fmt.Sprintf(" size: %s [a] · colour: %s [g] ", a.Measure, map[ColorMode]string{ColorType: "type", ColorAge: "age"}[a.Colors])
	rw := term.TextWidth(right)
	x := c.Text(0, 0, " ccdisk ", term.Style{Fg: 231, Bg: 24, Bold: true}, c.W)
	x += c.Text(x, 0, "│ ", sHeader, c.W-x)
	var crumbs []*scan.Node
	for p := a.View; p != nil; p = p.Parent {
		crumbs = append([]*scan.Node{p}, crumbs...)
	}
	limit := c.W - rw - 1
	// Drop leading crumbs (after the root) until the trail fits.
	width := func(cs []*scan.Node) int {
		w := 0
		for i, n := range cs {
			w += term.TextWidth(crumbName(n)) + map[bool]int{true: 3, false: 0}[i > 0]
		}
		return w
	}
	elided := false
	for len(crumbs) > 2 && x+width(crumbs)+4 > limit {
		crumbs = append(crumbs[:1], crumbs[2:]...)
		elided = true
	}
	for i, n := range crumbs {
		if i > 0 {
			sep := " › "
			if i == 1 && elided {
				sep = " › … › "
			}
			x += c.Text(x, 0, sep, sHeader, limit-x)
		}
		name := crumbName(n)
		x0 := x
		x += c.Text(x, 0, name, sCrumb, limit-x)
		target := n
		a.hits = append(a.hits, hit{x0, x, 0, func(bool) { a.Open(target) }})
	}
	if c.W-rw > x {
		c.Text(c.W-rw, 0, right, sHeader, rw)
	}
}

func crumbName(n *scan.Node) string {
	if n.Parent == nil {
		return filepath.Base(n.Name)
	}
	return n.Name
}

func (a *App) drawMap(c *term.Canvas) {
	r := a.mapRect
	if len(a.boxes) == 0 {
		c.Text(r.X+1, r.Y+1, "(empty: nothing here uses any "+a.Measure.String()+")", sDim, r.W-2)
	}
	for i := range a.boxes {
		a.drawBox(c, &a.boxes[i])
	}
	if a.Sel == nil {
		return
	}
	b := treemap.Nearest(a.boxes, a.Sel)
	if b == nil {
		return
	}
	if b.Node == a.Sel && b.Kind == treemap.DirBox {
		drawFrame(c, b.R, sSelFrame, true)
		a.drawTitle(c, b, sSelFrame)
		return
	}
	// A file, a frameless directory, or the box that hides the selection.
	for y := b.R.Y; y < b.R.Y+b.R.H; y++ {
		for x := b.R.X; x < b.R.X+b.R.W; x++ {
			c.SetStyle(x, y, term.Style{Fg: 16, Bg: 231, Bold: true})
		}
	}
}

func (a *App) drawBox(c *term.Canvas, b *treemap.Box) {
	r := b.R
	if b.Kind == treemap.GroupBox {
		c.Fill(r.X, r.Y, r.W, r.H, '░', sGroup)
		label := fmt.Sprintf("%d smaller", len(b.Group))
		if r.W >= 4 {
			c.Text(r.X, r.Y, term.Truncate(label, r.W), st(250, 235), r.W)
		}
		return
	}
	n := b.Node
	staged := a.Staging.Covered(n)
	_, reclaim := a.reclaimable(n)
	if b.Kind == treemap.DirBox {
		s := sFrame
		if a.Colors == ColorAge {
			s = st(ageColours[analysis.BandOf(n.Newest, a.Now())].bg, -1)
		}
		if reclaim {
			s = sReclaim
		}
		if staged {
			s = sStaged
		}
		c.Fill(r.X, r.Y, r.W, r.H, ' ', sNormal)
		drawFrame(c, r, s, false)
		a.drawTitle(c, b, s)
		return
	}
	col := a.fileColour(n)
	s := st(col.fg, col.bg)
	ch := ' '
	if n.Kind == scan.Dir {
		ch = '▒' // a directory too small to show its contents
		s = st(col.bg, 236)
	}
	if staged {
		s, ch = st(210, 52), '╳'
	}
	// Leave a dark gutter on the right and bottom, so neighbours of the same
	// colour stay distinct rectangles.
	fw, fh := r.W, r.H
	if fw >= 3 {
		fw--
	}
	if fh >= 3 {
		fh--
	}
	c.Fill(r.X, r.Y, r.W, r.H, ' ', sNormal)
	c.Fill(r.X, r.Y, fw, fh, ch, s)
	r.W = fw
	label := n.Name
	if n.Kind == scan.Symlink {
		label += " →"
	}
	if staged {
		label = "✗ " + label
	} else if reclaim {
		label = "♻ " + label
	}
	ls := term.Style{Fg: s.Fg, Bg: s.Bg, Bold: true}
	if n.Kind == scan.Dir && !staged {
		ls = st(231, 236)
	}
	c.Text(r.X, r.Y, term.Truncate(label, r.W), ls, r.W)
	if r.H >= 2 {
		c.Text(r.X, r.Y+1, term.Truncate(analysis.HumanSize(n.Size(a.Measure)), r.W), st(ls.Fg, ls.Bg), r.W)
	}
}

func (a *App) drawTitle(c *term.Canvas, b *treemap.Box, s term.Style) {
	n := b.Node
	title := " " + n.Name + " " + analysis.HumanSize(n.Size(a.Measure)) + " "
	if a.Staging.Covered(n) {
		title = " ✗" + title
	} else if _, ok := a.reclaimable(n); ok {
		title = " ♻" + title
	}
	if b.R.W > 4 {
		c.Text(b.R.X+1, b.R.Y, term.Truncate(title, b.R.W-2), term.Style{Fg: s.Fg, Bg: s.Bg, Bold: true}, b.R.W-2)
	}
}

func drawFrame(c *term.Canvas, r treemap.Rect, s term.Style, double bool) {
	h, v, tl, tr, bl, br := '─', '│', '┌', '┐', '└', '┘'
	if double {
		h, v, tl, tr, bl, br = '═', '║', '╔', '╗', '╚', '╝'
	}
	x1, y1 := r.X+r.W-1, r.Y+r.H-1
	for x := r.X + 1; x < x1; x++ {
		c.Set(x, r.Y, h, s)
		c.Set(x, y1, h, s)
	}
	for y := r.Y + 1; y < y1; y++ {
		c.Set(r.X, y, v, s)
		c.Set(x1, y, v, s)
	}
	c.Set(r.X, r.Y, tl, s)
	c.Set(x1, r.Y, tr, s)
	c.Set(r.X, y1, bl, s)
	c.Set(x1, y1, br, s)
}

type writer struct {
	c       *term.Canvas
	x, y, w int
	bottom  int
}

func (w *writer) line(s string, st term.Style) int {
	if w.y >= w.bottom {
		return -1
	}
	w.c.Text(w.x, w.y, s, st, w.w)
	w.y++
	return w.y - 1
}

// wrap prints s over as many lines as it needs.
func (w *writer) wrap(s string, st term.Style) {
	for s != "" && w.y < w.bottom {
		cut, used := len(s), 0
		for i, r := range s {
			if used+term.RuneWidth(r) > w.w {
				cut = i
				break
			}
			used += term.RuneWidth(r)
		}
		w.line(s[:cut], st)
		s = s[cut:]
	}
}

func (a *App) drawPanel(c *term.Canvas, x0, y0, w, h int) {
	for y := y0; y < y0+h; y++ {
		c.Set(x0, y, '│', sFrame)
	}
	x, pw := x0+2, w-3
	// Tabs.
	tx := x
	for _, t := range []struct {
		p    Panel
		name string
	}{{PanelInfo, "Info [i]"}, {PanelLargest, "Largest [l]"}, {PanelWarnings, fmt.Sprintf("Warnings %d [w]", len(a.Warnings))}} {
		s := sDim
		if a.Panel == t.p {
			s = term.Style{Fg: 16, Bg: 250, Bold: true}
		}
		x0t := tx
		tx += c.Text(tx, y0, " "+t.name+" ", s, x+pw-tx)
		p := t.p
		a.hits = append(a.hits, hit{x0t, tx, y0, func(bool) { a.Panel, a.PanelIdx = p, 0 }})
	}
	if a.Focus {
		c.Text(x+pw-6, y0, "focus", sKey, 6)
	}
	wr := &writer{c: c, x: x, y: y0 + 2, w: pw, bottom: y0 + h}
	a.panelRows = a.panelRows[:0]
	switch a.Panel {
	case PanelInfo:
		a.drawInfo(wr)
	case PanelLargest:
		a.drawLargest(wr)
	case PanelWarnings:
		a.drawWarnings(wr)
	}
}

func pct(part, whole int64) string {
	if whole <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(part)/float64(whole))
}

func (a *App) drawInfo(w *writer) {
	n := a.Sel
	if n == nil {
		n = a.View
	}
	if n == nil {
		return
	}
	m := a.Measure
	name := crumbName(n)
	w.line(term.Truncate(name, w.w), sBold)
	kind := n.Kind.String()
	if n.Kind == scan.Symlink && n.Target != "" {
		kind += " → " + n.Target
	}
	w.line(term.Truncate(kind, w.w), sDim)
	w.wrap(n.Path(), sNormal)
	w.y++
	mark := func(on bool) string {
		if on {
			return "●"
		}
		return " "
	}
	w.line(fmt.Sprintf("%s Disk usage:    %s", mark(m == scan.DiskUsage), analysis.HumanSize(n.TotalUsage)), sNormal)
	w.line(fmt.Sprintf("%s Apparent size: %s", mark(m == scan.ApparentSize), analysis.HumanSize(n.TotalApparent)), sNormal)
	if p := n.Parent; p != nil {
		w.wrap(fmt.Sprintf("%s, %s of %s", analysis.HumanSize(n.Size(m)), pct(n.Size(m), p.Size(m)), p.DisplayPath()), sNormal)
	}
	w.line(fmt.Sprintf("%s of the whole scan", pct(n.Size(m), a.Root.Size(m))), sNormal)
	w.line("Modified: "+n.ModTime.Format("2006-01-02 15:04"), sNormal)
	if n.Kind == scan.Dir {
		band := analysis.BandOf(n.Newest, a.Now())
		w.line(fmt.Sprintf("Newest inside: %s (%s)", n.Newest.Format("2006-01-02"), analysis.BandNames[band]), sNormal)
		w.line(fmt.Sprintf("Files: %d   Items: %d", n.Files, n.Items), sNormal)
	}
	if n.Kind == scan.File && n.Links > 1 {
		if n.CountedUsage == 0 {
			w.wrap(fmt.Sprintf("Hard link (%d names): its space is counted under another name", n.Links), sWarn)
		} else {
			w.wrap(fmt.Sprintf("Hard link (%d names): space counted here, once", n.Links), sWarn)
		}
	}
	if cat, ok := analysis.Classify(n); ok {
		w.line("♻ Reclaimable: "+string(cat), sReclaim)
	} else if analysis.IsReclaimable(n) {
		w.line("♻ Inside a reclaimable folder", sReclaim)
	}
	if a.Staging.Has(n) {
		w.line("✗ Staged for removal", sStaged)
	} else if a.Staging.Covered(n) {
		w.line("✗ Inside a staged folder", sStaged)
	}
	if n.Err != nil {
		w.wrap("⚠ Could not read: "+scan.ErrText(n.Err), sWarn)
	}
	if n.Kind != scan.Dir || len(n.Children) == 0 {
		return
	}
	w.y++
	w.line("Largest inside (1-9 or click to select):", sDim)
	for i, ch := range analysis.LargestChildren(n, m) {
		if w.y >= w.bottom {
			break
		}
		a.panelRows = append(a.panelRows, ch)
		key := " "
		if i < 9 {
			key = fmt.Sprint(i + 1)
		}
		s := sNormal
		if a.Focus && i == a.PanelIdx {
			s = sSelRow
		}
		row := fmt.Sprintf("%s %8s %6s %s", key, analysis.HumanSize(ch.Size(m)), pct(ch.Size(m), n.Size(m)), ch.Name)
		y := w.line(term.Truncate(row, w.w), s)
		target := ch
		a.hits = append(a.hits, hit{w.x, w.x + w.w, y, func(bool) { a.Select(target) }})
	}
}

func (a *App) drawLargest(w *writer) {
	d := a.derived()
	w.wrap("Largest files in "+a.View.DisplayPath()+" by "+a.Measure.String(), sBold)
	size := "any size"
	if minSizes[a.MinSize] > 0 {
		size = "≥" + analysis.HumanSize(minSizes[a.MinSize])
	}
	y := w.line(fmt.Sprintf("[m] %s  [o] %s", size, ageNames[a.MinAge]), sKey)
	a.hits = append(a.hits, hit{w.x, w.x + 3 + len(size), y, func(bool) { a.handleRune('m') }},
		hit{w.x + 5 + len(size), w.x + w.w, y, func(bool) { a.handleRune('o') }})
	if len(d.largest) == 0 {
		w.line("(no files match)", sDim)
	}
	for i, f := range d.largest {
		if w.y >= w.bottom {
			break
		}
		a.panelRows = append(a.panelRows, f)
		s := sNormal
		if f == a.Sel {
			s = st(16, 250)
		}
		if a.Focus && i == a.PanelIdx {
			s = sSelRow
		}
		row := fmt.Sprintf("%3d %8s %s", i+1, analysis.HumanSize(f.Size(a.Measure)), f.RelPath(a.View))
		y := w.line(term.Truncate(row, w.w), s)
		target := f
		a.hits = append(a.hits, hit{w.x, w.x + w.w, y, func(bool) { a.Select(target) }})
	}
}

func (a *App) drawWarnings(w *writer) {
	if len(a.Warnings) == 0 {
		w.line("No warnings: everything was readable.", sGood)
		return
	}
	w.line(fmt.Sprintf("%d item(s) could not be read; the scan carried on:", len(a.Warnings)), sWarn)
	for _, wn := range a.Warnings {
		if w.y >= w.bottom {
			break
		}
		w.wrap("⚠ "+wn.String(), sNormal)
	}
}

func (a *App) drawTotals(c *term.Canvas, y int) {
	d := a.derived()
	m := a.Measure
	staged := a.Staging.Total(scan.DiskUsage)
	parts := []struct {
		s  string
		st term.Style
	}{
		{fmt.Sprintf(" ♻ reclaimable here: %s (%d)", analysis.HumanSize(analysis.ReclaimableTotal(d.reclaim, m)), len(d.reclaim)), sReclaim},
		{fmt.Sprintf(" │ ✗ staged: %s (%d)", analysis.HumanSize(staged), a.Staging.Len()), sStaged},
	}
	if a.FreeErr == nil {
		parts = append(parts, struct {
			s  string
			st term.Style
		}{fmt.Sprintf(" │ free: %s → %s after removal", analysis.HumanSize(a.Free), analysis.HumanSize(a.Free+staged)), sNormal})
	}
	x := 0
	for _, p := range parts {
		x += c.Text(x, y, p.s, p.st, c.W-x)
	}
	// Legend, right-aligned when it fits.
	var items []struct {
		name string
		c    colour
	}
	if a.Colors == ColorAge {
		for i, n := range analysis.BandNames {
			items = append(items, struct {
				name string
				c    colour
			}{n, ageColours[i]})
		}
	} else {
		items = typeLegend
	}
	lw := 0
	for _, it := range items {
		lw += term.TextWidth(it.name) + 3
	}
	if lx := c.W - lw - 1; lx > x+2 {
		for _, it := range items {
			c.Set(lx, y, '■', st(it.c.bg, -1))
			lx += 1 + c.Text(lx+1, y, " "+it.name+" ", sDim, c.W-lx)
		}
	}
}

func (a *App) drawStatus(c *term.Canvas, y int) {
	if a.Status != "" {
		c.Text(0, y, " "+a.Status, sWarn, c.W)
		return
	}
	help := " ←↑→↓ move  ⏎ open  ⌫ back  t top  [ ] up/down  space stage  c clean up  tab panel focus  r rescan  q quit"
	if a.Focus {
		help = " panel: ↑↓ choose  ⏎ select in treemap  tab/esc back to treemap  1-9 jump"
	}
	c.Text(0, y, help, sDim, c.W)
}

func (a *App) drawBusy(c *term.Canvas) {
	y := c.H/2 - 2
	c.Text(2, y, term.Truncate(a.Busy+"…", c.W-4), sBold, c.W-4)
	if p := a.Progress; p != nil {
		el := a.Now().Sub(a.started).Round(100 * time.Millisecond)
		c.Text(2, y+2, fmt.Sprintf("%d items · %d directories · %s found · %s",
			p.Items.Load(), p.Dirs.Load(), analysis.HumanSize(p.Bytes.Load()), el), sNormal, c.W-4)
	}
	c.Text(2, y+4, "q to stop", sDim, c.W-4)
}

func (a *App) drawReview(c *term.Canvas) {
	drawFrame(c, treemap.Rect{X: 0, Y: 0, W: c.W, H: c.H}, sFrame, false)
	c.Text(2, 0, fmt.Sprintf(" Review: %d item(s) staged for removal ", len(a.review)), sBold, c.W-4)
	total := a.Staging.Total(scan.DiskUsage)
	w := &writer{c: c, x: 2, y: 2, w: c.W - 4, bottom: c.H - 6}
	w.line("Only these items will be touched. They go to "+a.Trasher.Where()+", not deleted.", sNormal)
	w.y++
	first := max(0, a.reviewIdx-(w.bottom-w.y)+1)
	for i := first; i < len(a.review) && w.y < w.bottom; i++ {
		n := a.review[i]
		s := sNormal
		if i == a.reviewIdx {
			s = sSelRow
		}
		note := ""
		if a.Staging.Covered(n.Parent) {
			note = "  (inside another staged item)"
		}
		row := fmt.Sprintf("✗ %9s  %-9s %s%s", analysis.HumanSize(analysis.StorageOf([]*scan.Node{n}, scan.DiskUsage)), n.Kind, n.DisplayPath(), note)
		y := w.line(term.Truncate(row, w.w), s)
		idx := i
		a.hits = append(a.hits, hit{w.x, w.x + w.w, y, func(bool) { a.reviewIdx = idx }})
	}
	y := c.H - 5
	c.Text(2, y, fmt.Sprintf("Total: %s (each piece of storage counted once)", analysis.HumanSize(total)), sBold, c.W-4)
	if a.FreeErr == nil {
		c.Text(2, y+1, fmt.Sprintf("Free space now %s, predicted after removal %s", analysis.HumanSize(a.Free), analysis.HumanSize(a.Free+total)), sNormal, c.W-4)
	}
	if a.confirm {
		c.Text(2, y+3, fmt.Sprintf("Move %d item(s), %s, to %s? Press y to confirm, any other key to cancel.", len(a.review), analysis.HumanSize(total), a.Trasher.Where()), term.Style{Fg: 16, Bg: 214, Bold: true}, c.W-4)
	} else {
		c.Text(2, y+3, "↑↓ choose   d/del drop from the list   y move to "+a.Trasher.Where()+"   esc back (nothing removed)", sKey, c.W-4)
	}
}

func (a *App) drawResults(c *term.Canvas) {
	r := a.Report
	drawFrame(c, treemap.Rect{X: 0, Y: 0, W: c.W, H: c.H}, sFrame, false)
	c.Text(2, 0, " Cleanup results ", sBold, c.W-4)
	w := &writer{c: c, x: 2, y: 2, w: c.W - 4, bottom: c.H - 2}
	w.line(fmt.Sprintf("Moved to %s: %d item(s), %s (scan estimate of what they held)",
		a.Trasher.Where(), r.Count(cleanup.Moved), analysis.HumanSize(r.Moved)), sGood)
	w.line(fmt.Sprintf("Skipped: %d   Failed: %d   (neither is counted as reclaimed)", r.Count(cleanup.Skipped), r.Count(cleanup.Failed)), sNormal)
	if r.FreeErr == nil {
		delta := r.FreeAfter - r.FreeBefore
		w.line(fmt.Sprintf("Free space reported by the filesystem: %s before, %s after (change %+d bytes, %s)",
			analysis.HumanSize(r.FreeBefore), analysis.HumanSize(r.FreeAfter), delta, signedHuman(delta)), sNormal)
		if r.Moved > 0 && delta < r.Moved/2 {
			w.wrap("Items in "+a.Trasher.Where()+" still use the disk: the space comes back when it is emptied.", sWarn)
		}
	}
	w.line(fmt.Sprintf("After rescanning, %s now holds %s.", a.Root.DisplayPath(), analysis.HumanSize(a.Root.TotalUsage)), sNormal)
	w.y++
	for _, o := range r.Outcomes {
		if w.y >= w.bottom {
			break
		}
		s := sGood
		switch o.Status {
		case cleanup.Skipped:
			s = sWarn
		case cleanup.Failed:
			s = sBad
		}
		line := fmt.Sprintf("%-7s %s", o.Status, o.Node.DisplayPath())
		if o.Reason != "" {
			line += " — " + o.Reason
		}
		w.wrap(line, s)
	}
	c.Text(2, c.H-1, " enter/esc: back to the treemap ", sKey, c.W-4)
}

func signedHuman(d int64) string {
	if d < 0 {
		return "-" + analysis.HumanSize(-d)
	}
	return "+" + analysis.HumanSize(d)
}
