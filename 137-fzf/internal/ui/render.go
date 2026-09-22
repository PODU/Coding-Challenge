package ui

import (
	"fmt"
	"strings"

	"ccfzf/internal/cli"
	"ccfzf/internal/item"
	"ccfzf/internal/matcher"
)

// The palette is deliberately limited to the basic SGR codes: they render the
// same on every terminal, including Windows consoles in VT mode.
const (
	colReset        = "\x1b[0m"
	colMatch        = "\x1b[32m"
	colMatchCurrent = "\x1b[1;32m"
	colCurrent      = "\x1b[1m"
	colPointer      = "\x1b[31m"
	colMarker       = "\x1b[31m"
	colInfo         = "\x1b[2m"
	colPrompt       = "\x1b[34m"
	colHeader       = "\x1b[36m"
)

var spinnerFrames = []string{"-", "\\", "|", "/"}

// geometry is where everything goes this frame. It is recomputed on every
// render so that a resize needs no special handling.
type geometry struct {
	top, height int

	listTop, listRows  int
	listLeft, listCols int

	promptRow int
	infoRow   int // 0 when the info line is inline or hidden
	headerTop int

	hasPreview                                     bool
	previewTop, previewLeft, previewRows, prevCols int
	previewSepRow, previewSepCol                   int
}

func (t *Terminal) chromeRows() int {
	n := 1 + len(t.opts.Header)
	if t.opts.Info == cli.InfoDefault {
		n++
	}
	return n
}

func (t *Terminal) geometry() geometry {
	g := geometry{top: t.origin, height: t.windowHeight()}
	left, width, rows := 1, t.cols, g.height
	top := g.top

	if t.preview != nil && !t.opts.PreviewWin.Hidden {
		pw := t.opts.PreviewWin
		switch pw.Position {
		case cli.PreviewRight, cli.PreviewLeft:
			size := pw.Size
			if pw.Percent {
				size = t.cols * pw.Size / 100
			}
			size = min(size, t.cols-10)
			if size > 3 {
				g.hasPreview = true
				g.previewTop, g.previewRows = top, rows
				g.prevCols = size - 2 // one column of separator, one of padding
				if pw.Position == cli.PreviewRight {
					g.previewSepCol = left + width - size
					g.previewLeft = g.previewSepCol + 2
				} else {
					g.previewLeft = left
					g.previewSepCol = left + size - 1
					left += size
				}
				width -= size
			}
		case cli.PreviewUp, cli.PreviewDown:
			size := pw.Size
			if pw.Percent {
				size = rows * pw.Size / 100
			}
			size = min(size, rows-t.chromeRows()-1)
			if size > 1 {
				g.hasPreview = true
				g.previewLeft, g.prevCols = left, width
				g.previewRows = size - 1
				if pw.Position == cli.PreviewUp {
					g.previewTop = top
					g.previewSepRow = top + size - 1
					top += size
				} else {
					g.previewSepRow = top + rows - size
					g.previewTop = g.previewSepRow + 1
				}
				rows -= size
			}
		}
	}

	g.listLeft, g.listCols = left, width
	g.listRows = max(rows-t.chromeRows(), 0)

	if t.opts.Layout == cli.LayoutReverse {
		g.promptRow = top
		next := top + 1
		if t.opts.Info == cli.InfoDefault {
			g.infoRow = next
			next++
		}
		g.headerTop = next
		g.listTop = next + len(t.opts.Header)
	} else {
		g.promptRow = top + rows - 1
		next := g.promptRow
		if t.opts.Info == cli.InfoDefault {
			next--
			g.infoRow = next
		}
		g.headerTop = next - len(t.opts.Header)
		g.listTop = top
	}
	return g
}

func (t *Terminal) render() {
	g := t.geometry()
	t.clampCursor()
	if t.preview != nil {
		t.preview.request(t.currentPreviewCommand())
	}

	t.tty.HideCursor()
	t.renderList(g)
	t.renderHeader(g)
	t.renderInfo(g)
	t.renderPrompt(g)
	if g.hasPreview {
		t.renderPreview(g)
	}

	// Leave the cursor in the query, where the user is typing.
	promptWidth := displayWidth(t.opts.Prompt)
	t.tty.MoveTo(g.promptRow, g.listLeft+promptWidth+displayWidth(string(t.query[:t.cursor])))
	t.tty.ShowCursor()
	t.tty.Flush()
}

// row returns the screen row for the p-th visible match. The default layout
// grows upward from the prompt, so the best match sits at the bottom.
func (g geometry) row(p int, layout cli.Layout) int {
	if layout == cli.LayoutDefault {
		return g.listTop + g.listRows - 1 - p
	}
	return g.listTop + p
}

func (t *Terminal) renderList(g geometry) {
	for p := range g.listRows {
		row := g.row(p, t.opts.Layout)
		idx := t.offset + p
		if idx >= len(t.matches) {
			t.writeLine(row, g.listLeft, "", g.listCols)
			continue
		}
		t.writeLine(row, g.listLeft, t.itemLine(t.matches[idx], idx == t.cy, g.listCols), g.listCols)
	}
}

// itemLine renders one row: pointer, multi-select marker, and the item text
// with the matched characters highlighted.
func (t *Terminal) itemLine(m matcher.Match, current bool, width int) string {
	var sb strings.Builder
	used := 0

	if current {
		sb.WriteString(colPointer + t.opts.Pointer + colReset)
	} else {
		sb.WriteString(strings.Repeat(" ", displayWidth(t.opts.Pointer)))
	}
	used += displayWidth(t.opts.Pointer)
	sb.WriteString(" ")
	used++

	if t.opts.Multi != 0 {
		if t.marked[m.Item.Index] {
			sb.WriteString(colMarker + t.opts.Marker + colReset)
		} else {
			sb.WriteString(strings.Repeat(" ", displayWidth(t.opts.Marker)))
		}
		used += displayWidth(t.opts.Marker)
	}

	text, _ := t.highlight(m, current, width-used)
	sb.WriteString(text)
	return sb.String()
}

// highlight walks the item text emitting a colour change only where the style
// actually changes, so a long line does not turn into a wall of escapes.
func (t *Terminal) highlight(m matcher.Match, current bool, width int) (string, int) {
	runes := []rune(m.Item.Display())
	var positions map[int]bool
	if t.pat != nil && !t.pat.IsEmpty() {
		if pos := matcher.Positions(t.pat, m.Item, t.slab); len(pos) > 0 {
			positions = make(map[int]bool, len(pos))
			for _, p := range pos {
				positions[p] = true
			}
		}
	}

	matchColor := colMatch
	base := ""
	if current {
		matchColor = colMatchCurrent
		base = colCurrent
	}

	// Reserve a column for the ellipsis before laying the text out, so the
	// mark cannot be squeezed off the end of the row it belongs to.
	limit := width
	truncated := displayWidth(m.Item.Display()) > width
	if truncated {
		limit = width - 1
	}

	var sb strings.Builder
	sb.WriteString(base)
	style := base
	used := 0
	for i, r := range runes {
		w := runeWidth(r)
		if used+w > limit {
			break
		}
		want := base
		switch {
		case positions[i]:
			want = matchColor
		default:
			if code := spanAt(m.Item.Colors, i); code != "" {
				want = "\x1b[" + code + "m"
			}
		}
		if want != style {
			sb.WriteString(colReset)
			sb.WriteString(want)
			style = want
		}
		sb.WriteRune(r)
		used += w
	}
	sb.WriteString(colReset)
	if truncated {
		// Mark the cut so a long path is not silently mistaken for a short one.
		sb.WriteString("…")
		used++
	}
	return sb.String(), used
}

func spanAt(spans []item.Span, i int) string {
	for _, s := range spans {
		if i >= s.Start && i < s.End {
			return s.Code
		}
	}
	return ""
}

func (t *Terminal) renderHeader(g geometry) {
	for i, line := range t.opts.Header {
		t.writeLine(g.headerTop+i, g.listLeft, colHeader+clip(line, g.listCols)+colReset, g.listCols)
	}
}

func (t *Terminal) infoText() string {
	s := fmt.Sprintf("%d/%d", len(t.matches), t.store.Count())
	if t.opts.Multi != 0 && len(t.marks) > 0 {
		s += fmt.Sprintf(" (%d)", len(t.marks))
	}
	if t.matchedUpTo < t.store.Count() || t.running {
		s += " …"
	}
	return s
}

func (t *Terminal) busy() bool { return !t.store.Done() || t.running }

func (t *Terminal) renderInfo(g geometry) {
	if t.opts.Info != cli.InfoDefault {
		return
	}
	prefix := "  "
	if t.busy() {
		prefix = spinnerFrames[t.spinner%len(spinnerFrames)] + " "
	}
	t.writeLine(g.infoRow, g.listLeft, colInfo+clip(prefix+t.infoText(), g.listCols)+colReset, g.listCols)
}

func (t *Terminal) renderPrompt(g geometry) {
	line := colPrompt + t.opts.Prompt + colReset + string(t.query)
	if t.opts.Info == cli.InfoInline {
		spinner := ""
		if t.busy() {
			spinner = spinnerFrames[t.spinner%len(spinnerFrames)] + " "
		}
		line += colInfo + "  " + spinner + t.infoText() + colReset
	}
	t.writeLine(g.promptRow, g.listLeft, line, g.listCols)
}

func (t *Terminal) renderPreview(g geometry) {
	lines := t.preview.snapshot()
	for i := range g.previewRows {
		row := g.previewTop + i
		text := ""
		if i < len(lines) {
			text = clipAnsi(strings.ReplaceAll(lines[i], "\t", "    "), g.prevCols)
		}
		t.writeLine(row, g.previewLeft, text, g.prevCols)
	}
	switch t.opts.PreviewWin.Position {
	case cli.PreviewLeft, cli.PreviewRight:
		for i := range g.previewRows {
			t.tty.MoveTo(g.previewTop+i, g.previewSepCol)
			t.tty.Write(colInfo + "│" + colReset)
		}
	case cli.PreviewUp, cli.PreviewDown:
		t.writeLine(g.previewSepRow, g.previewLeft, colInfo+strings.Repeat("─", g.prevCols)+colReset, g.prevCols)
	}
}

// writeLine places text at (row, col) and pads it to width, which keeps
// neighbouring panes intact - clearing the whole terminal line would wipe the
// preview sitting next to the list.
func (t *Terminal) writeLine(row, col int, text string, width int) {
	if row < 1 || row > t.rows || width <= 0 {
		return
	}
	t.tty.MoveTo(row, col)
	t.tty.Write(text)
	if pad := width - visibleWidth(text); pad > 0 {
		t.tty.Write(strings.Repeat(" ", pad))
	}
}

// runeWidth is a cheap approximation of terminal cell width: enough to keep
// CJK text and emoji from shifting the columns, without a full Unicode table.
func runeWidth(r rune) int {
	switch {
	case r < 0x1100:
		return 1
	case r >= 0x1100 && r <= 0x115f, // Hangul Jamo
		r >= 0x2e80 && r <= 0xa4cf, // CJK radicals .. Yi
		r >= 0xac00 && r <= 0xd7a3, // Hangul syllables
		r >= 0xf900 && r <= 0xfaff, // CJK compatibility ideographs
		r >= 0xfe30 && r <= 0xfe6f, // CJK compatibility forms
		r >= 0xff00 && r <= 0xff60, // fullwidth forms
		r >= 0xffe0 && r <= 0xffe6,
		r >= 0x1f300 && r <= 0x1f64f, // emoji
		r >= 0x1f900 && r <= 0x1f9ff,
		r >= 0x20000 && r <= 0x3fffd:
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// visibleWidth is displayWidth ignoring escape sequences.
func visibleWidth(s string) int {
	w := 0
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
		case r == '\x1b':
			inEscape = true
		default:
			w += runeWidth(r)
		}
	}
	return w
}

func clip(s string, width int) string {
	if displayWidth(s) <= width {
		return s
	}
	var sb strings.Builder
	used := 0
	for _, r := range s {
		w := runeWidth(r)
		if used+w > width {
			break
		}
		sb.WriteRune(r)
		used += w
	}
	return sb.String()
}

// clipAnsi truncates to width while letting escape sequences through, so
// colourful preview output keeps its colours.
func clipAnsi(s string, width int) string {
	var sb strings.Builder
	used := 0
	inEscape := false
	sawEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			sb.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		case r == '\x1b':
			inEscape, sawEscape = true, true
			sb.WriteRune(r)
			continue
		}
		w := runeWidth(r)
		if used+w > width {
			break
		}
		sb.WriteRune(r)
		used += w
	}
	if sawEscape {
		sb.WriteString(colReset)
	}
	return sb.String()
}
