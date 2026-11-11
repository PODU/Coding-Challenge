// Package cleanup removes reviewed items by moving them to the system Trash.
// It never deletes anything permanently, re-checks every item immediately
// before moving it, and reports exactly what happened to each one.
package cleanup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"ccdisk/internal/analysis"
	"ccdisk/internal/scan"
)

// Trasher moves one path to a recoverable location.
type Trasher interface {
	Trash(path string) error
	// Where names the destination for messages ("the Recycle Bin").
	Where() string
}

type Status uint8

const (
	Moved Status = iota
	Skipped
	Failed
)

func (s Status) String() string {
	return [...]string{"moved", "skipped", "failed"}[s]
}

type Outcome struct {
	Node   *scan.Node
	Path   string
	Status Status
	Reason string // for skipped and failed items
}

// Report summarises a cleanup run. Moved is what the scan said the moved
// items held; FreeBefore/FreeAfter are what the filesystem says, so the two
// can honestly differ (Trash on the same volume frees nothing yet).
type Report struct {
	Outcomes   []Outcome
	Moved      int64
	FreeBefore int64
	FreeAfter  int64
	FreeErr    error
}

func (r Report) Count(s Status) int {
	n := 0
	for _, o := range r.Outcomes {
		if o.Status == s {
			n++
		}
	}
	return n
}

// Execute moves each reviewed item to the Trash, in order. Items inside
// another reviewed item go with it and are not handled separately.
func Execute(items []*scan.Node, pol analysis.Policy, t Trasher) Report {
	var r Report
	r.FreeBefore, _, r.FreeErr = scan.FreeSpace(pol.Root)
	reviewed := map[*scan.Node]bool{}
	for _, n := range items {
		reviewed[n] = true
	}
	var moved []*scan.Node
	for _, n := range items {
		if coveredByOther(n, reviewed) {
			continue
		}
		o := handle(n, pol, t)
		r.Outcomes = append(r.Outcomes, o)
		if o.Status == Moved {
			moved = append(moved, n)
		}
	}
	r.Moved = analysis.StorageOf(moved, scan.DiskUsage)
	if r.FreeErr == nil {
		r.FreeAfter, _, r.FreeErr = scan.FreeSpace(pol.Root)
	}
	return r
}

func coveredByOther(n *scan.Node, set map[*scan.Node]bool) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if set[p] {
			return true
		}
	}
	return false
}

func handle(n *scan.Node, pol analysis.Policy, t Trasher) Outcome {
	path := n.Path()
	o := Outcome{Node: n, Path: path}
	if reason := Recheck(n, pol); reason != "" {
		o.Status, o.Reason = Skipped, reason
		return o
	}
	if err := t.Trash(path); err != nil {
		o.Status, o.Reason = Failed, "could not move to "+t.Where()+": "+errText(err)
		return o
	}
	// Trust the filesystem, not the call: the item must really be gone.
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		o.Status, o.Reason = Failed, "still present after moving to "+t.Where()
		return o
	}
	o.Status = Moved
	return o
}

// Recheck returns why n must be skipped now, or "" if it is still the same
// item the user reviewed and still allowed. Skipping is always preferred to
// removing the wrong thing.
func Recheck(n *scan.Node, pol analysis.Policy) string {
	path := n.Path()
	if err := pol.Check(path); err != nil {
		reason, _ := analysis.IsRefused(err)
		return "not eligible: " + reason
	}
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "no longer exists"
	}
	if err != nil {
		return "cannot check it: " + errText(err)
	}
	if k := scan.KindOf(fi); k != n.Kind {
		return fmt.Sprintf("changed from a %s to a %s since the scan", n.Kind, k)
	}
	if n.ID.Valid() {
		if id := scan.Identify(path, fi); id.Valid() && id != n.ID {
			return "replaced by a different " + n.Kind.String() + " since the scan"
		}
	}
	return ""
}

func errText(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err.Error()
	}
	return err.Error()
}
