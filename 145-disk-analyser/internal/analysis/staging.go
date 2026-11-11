package analysis

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"ccdisk/internal/scan"
)

// Policy decides what may be staged or removed at all.
type Policy struct {
	Root string
	// Subtrees may not be touched, nor anything inside them.
	Subtrees []string
	// Exact paths may not be touched themselves (their contents may).
	Exact []string
	// Anything that contains a Subtree or Exact path is refused too, so
	// staging "/" or "C:\" is refused even when scanning from there.
	CaseInsensitive bool
}

// DefaultPolicy protects system directories and the home directory itself.
func DefaultPolicy(root string) Policy {
	p := Policy{Root: root, CaseInsensitive: runtime.GOOS == "windows" || runtime.GOOS == "darwin"}
	if runtime.GOOS == "windows" {
		sys := os.Getenv("SystemRoot")
		if sys == "" {
			sys = `C:\Windows`
		}
		p.Subtrees = []string{sys, `C:\Windows`, `C:\Program Files`, `C:\Program Files (x86)`, `C:\ProgramData`}
		p.Exact = []string{`C:\Users`}
		for c := 'A'; c <= 'Z'; c++ {
			p.Exact = append(p.Exact, string(c)+`:\`)
		}
	} else {
		p.Subtrees = []string{"/System", "/usr", "/bin", "/sbin", "/etc", "/boot", "/lib", "/lib32",
			"/lib64", "/libx32", "/dev", "/proc", "/sys", "/private/etc", "/private/var/db", "/Library", "/Applications"}
		p.Exact = []string{"/", "/home", "/Users", "/var", "/opt", "/private", "/Volumes", "/mnt", "/media"}
	}
	if home, err := os.UserHomeDir(); err == nil {
		p.Exact = append(p.Exact, home)
	}
	return p
}

type RefusedError struct {
	Path   string
	Reason string
}

func (e *RefusedError) Error() string { return e.Path + ": " + e.Reason }

// Check returns a *RefusedError if path may not be staged or removed.
func (p Policy) Check(path string) error {
	clean := filepath.Clean(path)
	refuse := func(reason string) error { return &RefusedError{clean, reason} }
	if !p.inside(clean, p.Root) {
		return refuse("outside the scanned root " + p.Root)
	}
	if p.same(clean, p.Root) {
		return refuse("is the scan root itself")
	}
	for _, s := range p.Subtrees {
		if p.inside(clean, s) {
			return refuse("is a protected system location (" + s + ")")
		}
		if p.inside(s, clean) {
			return refuse("contains the protected system location " + s)
		}
	}
	for _, s := range p.Exact {
		if p.same(clean, s) {
			return refuse("is a protected location")
		}
		if p.inside(s, clean) {
			return refuse("contains the protected location " + s)
		}
	}
	return nil
}

func (p Policy) norm(s string) string {
	s = filepath.Clean(s)
	if p.CaseInsensitive {
		s = strings.ToLower(s)
	}
	return s
}

func (p Policy) same(a, b string) bool { return p.norm(a) == p.norm(b) }

// inside reports whether a is b or below it.
func (p Policy) inside(a, b string) bool {
	rel, err := filepath.Rel(p.norm(b), p.norm(a))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// Staging is the set of items marked for removal. It is only a set in
// memory; nothing touches the disk until cleanup.Execute.
type Staging struct {
	Policy Policy
	set    map[*scan.Node]bool
}

func NewStaging(p Policy) *Staging { return &Staging{Policy: p, set: map[*scan.Node]bool{}} }

func (s *Staging) Has(n *scan.Node) bool { return s.set[n] }

// Covered reports whether n or an ancestor is staged.
func (s *Staging) Covered(n *scan.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if s.set[p] {
			return true
		}
	}
	return false
}

func (s *Staging) Add(n *scan.Node) error {
	if err := s.Policy.Check(n.Path()); err != nil {
		return err
	}
	s.set[n] = true
	return nil
}

func (s *Staging) Remove(n *scan.Node) { delete(s.set, n) }

// Toggle stages or unstages n, returning whether it is now staged.
func (s *Staging) Toggle(n *scan.Node) (bool, error) {
	if s.set[n] {
		delete(s.set, n)
		return false, nil
	}
	return true, s.Add(n)
}

func (s *Staging) Len() int { return len(s.set) }

func (s *Staging) Clear() { s.set = map[*scan.Node]bool{} }

// Items lists the staged nodes by path.
func (s *Staging) Items() []*scan.Node {
	out := make([]*scan.Node, 0, len(s.set))
	for n := range s.set {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path() < out[j].Path() })
	return out
}

// Topmost drops items that sit inside another staged item.
func (s *Staging) Topmost() []*scan.Node {
	var out []*scan.Node
	for _, n := range s.Items() {
		if n.Parent == nil || !s.Covered(n.Parent) {
			out = append(out, n)
		}
	}
	return out
}

// Total is the space the staged set holds, each piece of storage once: an
// item inside a staged directory is not added again, and two staged names
// of one hard-linked file count once.
func (s *Staging) Total(m scan.Measure) int64 { return StorageOf(s.Topmost(), m) }

// StorageOf sums the storage under nodes, counting each hard-linked file
// once. It uses raw sizes rather than the scan's counted sizes, so staging
// only the second name of a hard-linked file still shows its size.
func StorageOf(nodes []*scan.Node, m scan.Measure) int64 {
	seen := map[scan.FileID]bool{}
	var total int64
	var walk func(n *scan.Node)
	walk = func(n *scan.Node) {
		if n.Kind != scan.Dir && n.Links > 1 && n.ID.Valid() {
			if seen[n.ID] {
				return
			}
			seen[n.ID] = true
		}
		if m == scan.ApparentSize {
			total += n.Apparent
		} else {
			total += n.Usage
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return total
}

// Forget drops staged nodes that are no longer in the tree (after a rescan
// replaced them).
func (s *Staging) Forget(root *scan.Node) {
	for n := range s.set {
		if n.Root() != root || !attached(n) {
			delete(s.set, n)
		}
	}
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

// IsRefused reports whether err is a policy refusal, and its reason.
func IsRefused(err error) (string, bool) {
	var r *RefusedError
	if errors.As(err, &r) {
		return r.Reason, true
	}
	return "", false
}

// HumanSize formats bytes with binary units, as du -h does.
func HumanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	f := float64(b)
	for _, u := range []string{"KB", "MB", "GB", "TB", "PB"} {
		f /= unit
		if f < unit || u == "PB" {
			if f >= 100 {
				return fmt.Sprintf("%.0f %s", f, u)
			}
			return fmt.Sprintf("%.1f %s", f, u)
		}
	}
	return ""
}
