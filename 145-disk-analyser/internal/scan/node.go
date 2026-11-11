// Package scan walks a directory tree into memory and measures it. It never
// opens anything for writing: every call it makes is a stat, a directory
// listing, a readlink, or (on Windows) an attribute-only handle.
package scan

import (
	"path/filepath"
	"strings"
	"time"
)

type Kind uint8

const (
	File Kind = iota
	Dir
	Symlink // symlinks, and on Windows junctions and other reparse points
	Other   // devices, sockets, pipes
)

func (k Kind) String() string {
	switch k {
	case File:
		return "file"
	case Dir:
		return "directory"
	case Symlink:
		return "link"
	default:
		return "special file"
	}
}

// Measure selects which size is shown. Both are always computed.
type Measure uint8

const (
	DiskUsage    Measure = iota // allocated space, what du reports
	ApparentSize                // file length, what ls -l reports
)

func (m Measure) String() string {
	if m == ApparentSize {
		return "apparent size"
	}
	return "disk usage"
}

// FileID identifies the storage behind a name: device and inode on Unix,
// volume serial and file index on Windows. The zero value means unknown.
type FileID struct{ Dev, Ino uint64 }

func (id FileID) Valid() bool { return id != FileID{} }

type Node struct {
	Name     string // the full root path for the root node
	Parent   *Node
	Kind     Kind
	Children []*Node // directories only, sorted by name
	Apparent int64
	Usage    int64
	ModTime  time.Time
	ID       FileID
	Links    uint32 // number of hard links; >1 means the storage may be shared
	Target   string // symlink target
	Err      error  // set when a directory could not be (fully) read

	// Filled in by Recompute.
	CountedUsage    int64 // Usage, or 0 when another name of the same file owns it
	CountedApparent int64
	TotalUsage      int64 // directories: summed over descendants, hard links once
	TotalApparent   int64
	Files           int       // regular file names under (or at) this node
	Items           int       // descendants, not counting the node itself
	Newest          time.Time // directories: newest descendant mtime
}

func (n *Node) IsDir() bool { return n.Kind == Dir }

func (n *Node) Path() string {
	if n.Parent == nil {
		return n.Name
	}
	return filepath.Join(n.Parent.Path(), n.Name)
}

// Size is the node's total in the chosen measure.
func (n *Node) Size(m Measure) int64 {
	if m == ApparentSize {
		return n.TotalApparent
	}
	return n.TotalUsage
}

func (n *Node) Root() *Node {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}

// Within reports whether n is anc or one of its descendants.
func (n *Node) Within(anc *Node) bool {
	for ; n != nil; n = n.Parent {
		if n == anc {
			return true
		}
	}
	return false
}

func (n *Node) Depth() int {
	d := 0
	for p := n.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

// DisplayPath is the path relative to the scan root's parent, so the root
// shows as its own base name ("testroot/media").
func (n *Node) DisplayPath() string {
	if n.Parent == nil {
		return filepath.Base(n.Name)
	}
	return filepath.Join(n.Parent.DisplayPath(), n.Name)
}

// RelPath is the path below base, or "." for base itself.
func (n *Node) RelPath(base *Node) string {
	if n == base {
		return "."
	}
	var parts []string
	for p := n; p != nil && p != base; p = p.Parent {
		parts = append(parts, p.Name)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return filepath.Join(parts...)
}

// Find returns the node at a path relative to n, or nil.
func (n *Node) Find(rel string) *Node {
	cur := n
	for _, part := range splitPath(rel) {
		var next *Node
		for _, c := range cur.Children {
			if c.Name == part {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

func splitPath(p string) []string {
	var parts []string
	for _, s := range strings.Split(filepath.ToSlash(filepath.Clean(p)), "/") {
		if s != "" && s != "." {
			parts = append(parts, s)
		}
	}
	return parts
}

// Recompute fills in the totals. It does no I/O, so it is cheap to rerun
// after any part of the tree is rescanned.
//
// A file with several hard links is counted once, under one owner name. The
// owner is the shallowest, then shortest, then alphabetically first name, so
// that an original like media/video.bin owns the space rather than a later
// media/video-link.bin.
func Recompute(root *Node) {
	owner := map[FileID]*Node{}
	var collect func(n *Node)
	collect = func(n *Node) {
		if n.Kind != Dir && n.Links > 1 && n.ID.Valid() {
			if o, ok := owner[n.ID]; !ok || ownsBefore(n, o) {
				owner[n.ID] = n
			}
		}
		for _, c := range n.Children {
			collect(c)
		}
	}
	collect(root)

	var walk func(n *Node)
	walk = func(n *Node) {
		if n.Kind != Dir {
			n.CountedUsage, n.CountedApparent = n.Usage, n.Apparent
			if o, ok := owner[n.ID]; ok && o != n && n.Links > 1 {
				n.CountedUsage, n.CountedApparent = 0, 0
			}
			n.TotalUsage, n.TotalApparent = n.CountedUsage, n.CountedApparent
			n.Files, n.Items = 0, 0
			if n.Kind == File {
				n.Files = 1
			}
			n.Newest = n.ModTime
			return
		}
		n.CountedUsage, n.CountedApparent = n.Usage, n.Apparent
		n.TotalUsage, n.TotalApparent = n.Usage, n.Apparent
		n.Files, n.Items = 0, 0
		n.Newest = time.Time{}
		for _, c := range n.Children {
			walk(c)
			n.TotalUsage += c.TotalUsage
			n.TotalApparent += c.TotalApparent
			n.Files += c.Files
			n.Items += 1 + c.Items
			if c.Newest.After(n.Newest) {
				n.Newest = c.Newest
			}
		}
		if n.Newest.IsZero() {
			n.Newest = n.ModTime
		}
	}
	walk(root)
}

func ownsBefore(a, b *Node) bool {
	da, db := a.Depth(), b.Depth()
	if da != db {
		return da < db
	}
	if len(a.Name) != len(b.Name) {
		return len(a.Name) < len(b.Name)
	}
	return a.Path() < b.Path()
}
