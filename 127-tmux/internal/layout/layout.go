// Package layout works out where each pane of a window goes.
//
// Panes are the leaves of a binary tree whose inner nodes are splits. Each
// split gives one row or column to the divider between its two halves.
package layout

import "sort"

type Orientation int

const (
	Horizontal Orientation = iota // one above the other
	Vertical                      // side by side
)

type Direction int

const (
	Up Direction = iota
	Down
	Left
	Right
)

type Rect struct {
	X, Y, W, H int
}

func (r Rect) Contains(cx, cy int) bool {
	return cx >= r.X && cx < r.X+r.W && cy >= r.Y && cy < r.Y+r.H
}

// node is either a pane (no children) or a split.
type node struct {
	paneID   int
	orient   Orientation
	ratio    float64 // share of the space that goes to the first child
	children [2]*node
}

func (n *node) isLeaf() bool { return n.children[0] == nil }

// split divides r between n's two children and returns their rectangles and
// the divider between them.
func (n *node) split(r Rect) (first, second, divider Rect) {
	if n.orient == Horizontal {
		usable := max(r.H-1, 0)
		top := int(float64(usable)*n.ratio + 0.5)
		return Rect{r.X, r.Y, r.W, top},
			Rect{r.X, r.Y + top + 1, r.W, usable - top},
			Rect{r.X, r.Y + top, r.W, 1}
	}
	usable := max(r.W-1, 0)
	left := int(float64(usable)*n.ratio + 0.5)
	return Rect{r.X, r.Y, left, r.H},
		Rect{r.X + left + 1, r.Y, usable - left, r.H},
		Rect{r.X + left, r.Y, 1, r.H}
}

// Tree is the pane layout of one window.
type Tree struct {
	root   *node
	active int
	nextID int
	zoom   bool
}

// New returns a tree holding a single pane with id 0.
func New() *Tree {
	return &Tree{root: &node{paneID: 0}, nextID: 1}
}

func (t *Tree) Active() int { return t.active }

// SetActive makes pane id the active pane if it exists.
func (t *Tree) SetActive(id int) bool {
	if t.find(t.root, id) != nil {
		t.active = id
		return true
	}
	return false
}

func (t *Tree) Zoomed() bool { return t.zoom }

func (t *Tree) ToggleZoom() { t.zoom = !t.zoom }

// PaneIDs lists the panes in tree order (left before right, top before bottom).
func (t *Tree) PaneIDs() []int {
	var ids []int
	var walk func(n *node)
	walk = func(n *node) {
		if n.isLeaf() {
			ids = append(ids, n.paneID)
			return
		}
		walk(n.children[0])
		walk(n.children[1])
	}
	walk(t.root)
	return ids
}

func (t *Tree) Count() int { return len(t.PaneIDs()) }

// Split halves the active pane and makes the new half (the second child)
// active. It returns the new pane's id.
func (t *Tree) Split(o Orientation) int {
	leaf := t.find(t.root, t.active)
	if leaf == nil {
		return -1
	}
	newID := t.nextID
	t.nextID++
	leaf.children[0] = &node{paneID: leaf.paneID}
	leaf.children[1] = &node{paneID: newID}
	leaf.paneID = -1
	leaf.orient = o
	leaf.ratio = 0.5
	t.active = newID
	t.zoom = false
	return newID
}

// Close removes pane id, letting its sibling take over the space. It returns
// false, and changes nothing, if id is the only pane left.
func (t *Tree) Close(id int) bool {
	if t.root.isLeaf() {
		return false
	}
	parent, idx := t.findParent(t.root, id)
	if parent == nil {
		return true
	}
	sibling := parent.children[1-idx]
	*parent = *sibling
	if t.find(t.root, t.active) == nil {
		if ids := t.PaneIDs(); len(ids) > 0 {
			t.active = ids[0]
		}
	}
	t.zoom = false
	return true
}

// Geometry returns each pane's rectangle within win. When zoomed, only the
// active pane is included and it gets all of win.
func (t *Tree) Geometry(win Rect) map[int]Rect {
	out := make(map[int]Rect)
	if t.zoom {
		out[t.active] = win
		return out
	}
	t.layout(t.root, win, out)
	return out
}

func (t *Tree) layout(n *node, r Rect, out map[int]Rect) {
	if n.isLeaf() {
		out[n.paneID] = r
		return
	}
	a, b, _ := n.split(r)
	t.layout(n.children[0], a, out)
	t.layout(n.children[1], b, out)
}

// Borders returns the dividers between panes. A zoomed window has none.
func (t *Tree) Borders(win Rect) []Border {
	var bs []Border
	if t.zoom {
		return bs
	}
	t.borders(t.root, win, &bs)
	return bs
}

// Border is one divider line, a single cell wide or tall.
type Border struct {
	Rect     Rect
	Vertical bool
}

func (t *Tree) borders(n *node, r Rect, bs *[]Border) {
	if n.isLeaf() {
		return
	}
	a, b, divider := n.split(r)
	*bs = append(*bs, Border{divider, n.orient == Vertical})
	t.borders(n.children[0], a, bs)
	t.borders(n.children[1], b, bs)
}

// Navigate makes the nearest pane in direction d active and returns its id.
// If there is no pane that way, the active pane stays as it is.
func (t *Tree) Navigate(win Rect, d Direction) int {
	geo := t.Geometry(win)
	cur, ok := geo[t.active]
	if !ok {
		return t.active
	}
	cx := cur.X + cur.W/2
	cy := cur.Y + cur.H/2

	type cand struct {
		id   int
		dist int
	}
	var cands []cand
	for id, r := range geo {
		if id == t.active {
			continue
		}
		var ok bool
		var dist int
		switch d {
		case Up:
			ok = r.Y+r.H <= cur.Y
			dist = cur.Y - (r.Y + r.H)
		case Down:
			ok = r.Y >= cur.Y+cur.H
			dist = r.Y - (cur.Y + cur.H)
		case Left:
			ok = r.X+r.W <= cur.X
			dist = cur.X - (r.X + r.W)
		case Right:
			ok = r.X >= cur.X+cur.W
			dist = r.X - (cur.X + cur.W)
		}
		if !ok {
			continue
		}
		// Among equally near panes, prefer the one best lined up with ours.
		var perp int
		if d == Up || d == Down {
			perp = abs(r.X + r.W/2 - cx)
		} else {
			perp = abs(r.Y + r.H/2 - cy)
		}
		cands = append(cands, cand{id, dist*1000 + perp})
	}
	if len(cands) == 0 {
		return t.active
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].dist < cands[j].dist })
	t.active = cands[0].id
	return t.active
}

// Resize moves the active pane's edge by delta cells, growing it at the
// expense of its neighbour along that axis. Neither side shrinks below one
// cell. It returns false if there is nothing to resize.
func (t *Tree) Resize(win Rect, d Direction, delta int) bool {
	want := Vertical
	if d == Up || d == Down {
		want = Horizontal
	}
	anc, idx, r := t.ancestor(t.root, win, t.active, want)
	if anc == nil {
		return false
	}
	var span int
	if want == Horizontal {
		span = r.H - 1
	} else {
		span = r.W - 1
	}
	if span <= 1 {
		return false
	}
	// ratio sizes the first child, so growing the second means lowering it.
	grow := delta
	if idx != 0 {
		grow = -delta
	}
	cur := int(anc.ratio*float64(span) + 0.5)
	next := cur + grow
	if next < 1 {
		next = 1
	}
	if next > span-1 {
		next = span - 1
	}
	anc.ratio = float64(next) / float64(span)
	return true
}

// ancestor finds the deepest split with orientation want above pane id. It
// returns the split, which child (0 or 1) the pane is under, and the split's
// rectangle.
func (t *Tree) ancestor(n *node, r Rect, id int, want Orientation) (*node, int, Rect) {
	if n == nil || n.isLeaf() {
		return nil, 0, Rect{}
	}
	c0, c1, _ := n.split(r)
	inChild := 0
	if t.find(n.children[1], id) != nil {
		inChild = 1
	} else if t.find(n.children[0], id) == nil {
		return nil, 0, Rect{}
	}
	childRect := c0
	if inChild == 1 {
		childRect = c1
	}
	if deep, di, dr := t.ancestor(n.children[inChild], childRect, id, want); deep != nil {
		return deep, di, dr
	}
	if n.orient == want {
		return n, inChild, r
	}
	return nil, 0, Rect{}
}

func (t *Tree) find(n *node, id int) *node {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		if n.paneID == id {
			return n
		}
		return nil
	}
	if r := t.find(n.children[0], id); r != nil {
		return r
	}
	return t.find(n.children[1], id)
}

func (t *Tree) findParent(n *node, id int) (*node, int) {
	if n == nil || n.isLeaf() {
		return nil, 0
	}
	for i := 0; i < 2; i++ {
		c := n.children[i]
		if c.isLeaf() && c.paneID == id {
			return n, i
		}
		if p, idx := t.findParent(c, id); p != nil {
			return p, idx
		}
	}
	return nil, 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
