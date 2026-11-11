package treemap

import (
	"fmt"
	"math"
	"testing"

	"ccdisk/internal/scan"
)

// tree builds an in-memory tree: a map value of int64 is a file size, a map
// is a directory.
func tree(name string, spec map[string]any) *scan.Node {
	root := &scan.Node{Name: name, Kind: scan.Dir}
	var add func(parent *scan.Node, spec map[string]any)
	add = func(parent *scan.Node, spec map[string]any) {
		for k, v := range spec {
			n := &scan.Node{Name: k, Parent: parent}
			switch v := v.(type) {
			case int64:
				n.Kind, n.Usage, n.Apparent = scan.File, v, v
			case int:
				n.Kind, n.Usage, n.Apparent = scan.File, int64(v), int64(v)
			case map[string]any:
				n.Kind = scan.Dir
				add(n, v)
			}
			parent.Children = append(parent.Children, n)
		}
	}
	add(root, spec)
	scan.Recompute(root)
	return root
}

const MB = 1 << 20

func media() *scan.Node {
	return tree("media", map[string]any{"video.bin": 40 * MB, "photo1.bin": 8 * MB, "photo2.bin": 2 * MB})
}

// checkTiling verifies the boxes directly under one parent cover r exactly.
func checkTiling(t *testing.T, boxes []Box, parent *scan.Node, r Rect) {
	t.Helper()
	grid := make([]int, r.W*r.H)
	for _, b := range boxes {
		if b.Parent != parent {
			continue
		}
		for y := b.R.Y; y < b.R.Y+b.R.H; y++ {
			for x := b.R.X; x < b.R.X+b.R.W; x++ {
				if !r.Contains(x, y) {
					t.Fatalf("box %v outside %v", b.R, r)
				}
				grid[(y-r.Y)*r.W+(x-r.X)]++
			}
		}
	}
	for i, c := range grid {
		if c != 1 {
			t.Fatalf("cell %d,%d covered %d times", r.X+i%r.W, r.Y+i/r.W, c)
		}
	}
}

func TestAreasProportional(t *testing.T) {
	m := media()
	r := Rect{0, 0, 120, 40}
	boxes := Layout(m, r, Options{})
	area := map[string]float64{}
	for _, b := range boxes {
		area[b.Node.Name] = float64(b.R.Area())
	}
	if ratio := area["video.bin"] / area["photo1.bin"]; math.Abs(ratio-5) > 0.5 {
		t.Errorf("video/photo1 = %.2f, want about 5", ratio)
	}
	if ratio := area["video.bin"] / area["photo2.bin"]; math.Abs(ratio-20) > 3 {
		t.Errorf("video/photo2 = %.2f, want about 20", ratio)
	}
	checkTiling(t, boxes, m, r)
}

func TestNoGapsOrOverlapsAtManySizes(t *testing.T) {
	spec := map[string]any{}
	for i := 0; i < 40; i++ {
		spec[fmt.Sprintf("f%02d", i)] = int64((i%7 + 1) * (i + 3) * 1000)
	}
	n := tree("x", spec)
	for w := 1; w < 90; w += 7 {
		for h := 1; h < 50; h += 5 {
			r := Rect{3, 2, w, h}
			checkTiling(t, Layout(n, r, Options{}), n, r)
		}
	}
}

func TestNestingAndFrames(t *testing.T) {
	root := tree("testroot", map[string]any{
		"media":    map[string]any{"video.bin": 40 * MB, "photo1.bin": 8 * MB, "photo2.bin": 2 * MB},
		"projects": map[string]any{"webapp": map[string]any{"big.js": 20 * MB}, "api": map[string]any{"api.bin": 3 * MB}},
		"old":      map[string]any{"archive.bin": 15 * MB},
	})
	r := Rect{0, 0, 100, 40}
	boxes := Layout(root, r, Options{})
	md := Find(boxes, root.Find("media"))
	if md == nil || md.Kind != DirBox {
		t.Fatal("media should be drawn as a container")
	}
	v := Find(boxes, root.Find("media/video.bin"))
	if v == nil || v.Depth != 1 {
		t.Fatal("video.bin should be drawn inside media")
	}
	in := md.Inner
	if v.R.X < in.X || v.R.Y < in.Y || v.R.X+v.R.W > in.X+in.W || v.R.Y+v.R.H > in.Y+in.H {
		t.Fatalf("video %v not inside media's frame %v", v.R, in)
	}
	checkTiling(t, boxes, root.Find("media"), in)
	if b := At(boxes, v.R.X, v.R.Y); b.Node != v.Node {
		t.Fatal("At should return the deepest box")
	}
}

func TestSmallItemsGrouped(t *testing.T) {
	spec := map[string]any{"big": 50 * MB}
	for i := 0; i < 500; i++ {
		spec[fmt.Sprintf("tiny%03d", i)] = 4096
	}
	n := tree("node_modules", spec)
	boxes := Layout(n, Rect{0, 0, 80, 24}, Options{})
	var groups []Box
	for _, b := range boxes {
		if b.Kind == GroupBox {
			groups = append(groups, b)
		}
	}
	if len(groups) != 1 || len(groups[0].Group) != 500 {
		t.Fatalf("want one group of 500, got %d groups", len(groups))
	}
	if len(boxes) != 2 {
		t.Fatalf("want big + group, got %d boxes", len(boxes))
	}
	if Nearest(boxes, n.Find("tiny007")) != &boxes[1] && Nearest(boxes, n.Find("tiny007")).Kind != GroupBox {
		t.Fatal("Nearest should find the group holding a small item")
	}
}

func TestZeroSizeAndHardLinkSkipped(t *testing.T) {
	n := tree("m", map[string]any{"video.bin": 40 * MB, "empty": 0})
	link := &scan.Node{Name: "video-link.bin", Parent: n, Kind: scan.File} // counted elsewhere: size 0
	n.Children = append(n.Children, link)
	boxes := Layout(n, Rect{0, 0, 40, 20}, Options{})
	if len(boxes) != 1 || boxes[0].Node.Name != "video.bin" {
		t.Fatalf("only video.bin should be drawn, got %d boxes", len(boxes))
	}
}

func TestSquarifiedAspect(t *testing.T) {
	vals := []float64{6, 6, 4, 3, 2, 2, 1}
	rs := squarify(vals, 0, 0, 6, 4)
	for i, r := range rs {
		if a := math.Max(r.w/r.h, r.h/r.w); a > 4 {
			t.Errorf("item %d aspect %.1f", i, a)
		}
		if math.Abs(r.w*r.h-vals[i]) > 1e-9 {
			t.Errorf("item %d area %.3f want %.0f", i, r.w*r.h, vals[i])
		}
	}
}
