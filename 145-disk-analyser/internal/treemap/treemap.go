// Package treemap lays a scanned tree out as nested rectangles on a grid of
// terminal cells, using the squarified algorithm (Bruls, Huizing and van Wijk)
// so rectangles stay close to square and are easy to compare by eye.
package treemap

import (
	"math"
	"sort"

	"ccdisk/internal/scan"
)

type Rect struct{ X, Y, W, H int }

func (r Rect) Area() int              { return r.W * r.H }
func (r Rect) Empty() bool            { return r.W <= 0 || r.H <= 0 }
func (r Rect) Contains(x, y int) bool { return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H }
func (r Rect) Center() (float64, float64) {
	return float64(r.X) + float64(r.W)/2, float64(r.Y) + float64(r.H)/2
}

type BoxKind uint8

const (
	FileBox  BoxKind = iota // files, links, and directories too small for a frame
	DirBox                  // a directory drawn as a frame around its children
	GroupBox                // "smaller items": children too small to draw one by one
)

type Box struct {
	Kind   BoxKind
	Node   *scan.Node   // nil for a group
	Parent *scan.Node   // the directory whose contents this box is part of
	Group  []*scan.Node // members of a group
	Size   int64
	R      Rect
	Inner  Rect // inside the frame, for directories drawn as frames
	Depth  int  // 0 for the opened directory's own children
}

type Options struct {
	Measure scan.Measure
	// MinCells is the area below which a child joins the "smaller items"
	// group rather than getting its own rectangle. Default 6.
	MinCells float64
	// CellAspect is a cell's height over its width. Terminal cells are about
	// twice as tall as wide; laying out in true proportions keeps the
	// rectangles visually square. Default 2.
	CellAspect float64
}

// Layout fills r with dir's contents. Boxes come in pre-order: a directory
// before its children, so the last box containing a point is the deepest.
func Layout(dir *scan.Node, r Rect, o Options) []Box {
	if o.MinCells <= 0 {
		o.MinCells = 6
	}
	if o.CellAspect <= 0 {
		o.CellAspect = 2
	}
	var out []Box
	layoutDir(&out, dir, r, 0, o)
	return out
}

type entry struct {
	node  *scan.Node
	group []*scan.Node
	size  int64
}

func layoutDir(out *[]Box, dir *scan.Node, r Rect, depth int, o Options) {
	if r.Empty() {
		return
	}
	var items []entry
	var total int64
	for _, c := range dir.Children {
		if s := c.Size(o.Measure); s > 0 {
			items = append(items, entry{node: c, size: s})
			total += s
		}
	}
	if total == 0 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].size > items[j].size })

	// Everything from the first child that would be too small to read is
	// merged into one region. Children are sorted, so the rest are smaller.
	cellsPerByte := float64(r.Area()) / float64(total)
	for k, it := range items {
		if float64(it.size)*cellsPerByte < o.MinCells {
			g := entry{}
			for _, s := range items[k:] {
				g.group = append(g.group, s.node)
				g.size += s.size
			}
			items = append(items[:k], g)
			break
		}
	}

	vals := make([]float64, len(items))
	scale := float64(r.W) * float64(r.H) * o.CellAspect / float64(total)
	for i, it := range items {
		vals[i] = float64(it.size) * scale
	}
	rects := squarify(vals, 0, 0, float64(r.W), float64(r.H)*o.CellAspect)

	for i, it := range items {
		f := rects[i]
		// Rounding edges, not sizes, means neighbours share the same integer
		// edge: no gaps and no overlaps.
		x0, x1 := r.X+iround(f.x), r.X+iround(f.x+f.w)
		y0, y1 := r.Y+iround(f.y/o.CellAspect), r.Y+iround((f.y+f.h)/o.CellAspect)
		x1, y1 = min(x1, r.X+r.W), min(y1, r.Y+r.H)
		b := Box{Node: it.node, Parent: dir, Group: it.group, Size: it.size,
			R: Rect{x0, y0, x1 - x0, y1 - y0}, Depth: depth}
		if b.R.Empty() {
			continue
		}
		switch {
		case it.group != nil:
			b.Kind = GroupBox
		case it.node.Kind == scan.Dir && b.R.W >= 3 && b.R.H >= 3:
			b.Kind = DirBox
			b.Inner = Rect{x0 + 1, y0 + 1, b.R.W - 2, b.R.H - 2}
		default:
			b.Kind = FileBox
		}
		*out = append(*out, b)
		if b.Kind == DirBox {
			layoutDir(out, it.node, b.Inner, depth+1, o)
		}
	}
}

func iround(f float64) int { return int(math.Round(f)) }

type frect struct{ x, y, w, h float64 }

// squarify places vals (sorted largest first, summing to w*h) in the
// rectangle. It adds items to the current row along the shorter side while
// that keeps the worst aspect ratio in the row from getting worse.
func squarify(vals []float64, x, y, w, h float64) []frect {
	out := make([]frect, len(vals))
	for i := 0; i < len(vals); {
		short := math.Min(w, h)
		j := i + 1
		best := worst(vals[i:j], short)
		for j < len(vals) {
			nw := worst(vals[i:j+1], short)
			if nw > best {
				break
			}
			best = nw
			j++
		}
		sum := 0.0
		for _, v := range vals[i:j] {
			sum += v
		}
		last := j == len(vals)
		if w >= h {
			sw := w // the final row takes all remaining space, absorbing rounding error
			if !last && h > 0 {
				sw = sum / h
			}
			yy := y
			for k := i; k < j; k++ {
				hh := 0.0
				if sum > 0 {
					hh = h * vals[k] / sum
				}
				out[k] = frect{x, yy, sw, hh}
				yy += hh
			}
			x, w = x+sw, w-sw
		} else {
			sh := h
			if !last && w > 0 {
				sh = sum / w
			}
			xx := x
			for k := i; k < j; k++ {
				ww := 0.0
				if sum > 0 {
					ww = w * vals[k] / sum
				}
				out[k] = frect{xx, y, ww, sh}
				xx += ww
			}
			y, h = y+sh, h-sh
		}
		i = j
	}
	return out
}

func worst(row []float64, side float64) float64 {
	sum, hi, lo := 0.0, 0.0, math.Inf(1)
	for _, v := range row {
		sum += v
		hi = math.Max(hi, v)
		lo = math.Min(lo, v)
	}
	if sum == 0 || lo == 0 || side == 0 {
		return math.Inf(1)
	}
	s2, sum2 := side*side, sum*sum
	return math.Max(s2*hi/sum2, sum2/(s2*lo))
}

// At returns the deepest box containing the cell, or nil.
func At(boxes []Box, x, y int) *Box {
	for i := len(boxes) - 1; i >= 0; i-- {
		if boxes[i].R.Contains(x, y) {
			return &boxes[i]
		}
	}
	return nil
}

// Find returns the box drawn for n, or nil if n is not drawn on its own
// (inside a "smaller items" group or below the drawable depth).
func Find(boxes []Box, n *scan.Node) *Box {
	for i := range boxes {
		if boxes[i].Node == n {
			return &boxes[i]
		}
	}
	return nil
}

// Nearest returns the box for n or, failing that, for its closest drawn
// ancestor, or the group that contains it.
func Nearest(boxes []Box, n *scan.Node) *Box {
	for p := n; p != nil; p = p.Parent {
		if b := Find(boxes, p); b != nil {
			return b
		}
		for i := range boxes {
			for _, m := range boxes[i].Group {
				if m == p {
					return &boxes[i]
				}
			}
		}
	}
	return nil
}
