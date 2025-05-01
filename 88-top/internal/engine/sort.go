package engine

import (
	"sort"
	"strings"

	"github.com/example/cctop/internal/model"
)

// sortProcs sorts procs by the view's column. Numeric columns go biggest
// first, as in top, and text columns and PID go A to Z; Reverse flips that.
// Ties are broken by PID, always ascending.
func sortProcs(procs []model.Process, view model.ViewState) {
	compare := compareFunc(view.Sort)
	descending := defaultDescending(view.Sort)
	if view.Reverse {
		descending = !descending
	}
	sort.SliceStable(procs, func(i, j int) bool {
		a, b := procs[i], procs[j]
		if c := compare(a, b); c != 0 {
			if descending {
				return c > 0
			}
			return c < 0
		}
		return a.PID < b.PID
	})
}

func defaultDescending(k model.SortKey) bool {
	switch k {
	case model.SortPID, model.SortUser, model.SortCommand:
		return false
	default:
		return true
	}
}

// compareFunc returns a function giving -1, 0 or 1 for the column k.
func compareFunc(k model.SortKey) func(a, b model.Process) int {
	switch k {
	case model.SortMem:
		return func(a, b model.Process) int { return cmpFloat(a.MemPercent, b.MemPercent) }
	case model.SortPID:
		return func(a, b model.Process) int { return cmpInt(a.PID, b.PID) }
	case model.SortTime:
		return func(a, b model.Process) int { return cmpFloat(a.CPUTime, b.CPUTime) }
	case model.SortUser:
		return func(a, b model.Process) int { return strings.Compare(a.User, b.User) }
	case model.SortCommand:
		return func(a, b model.Process) int {
			return strings.Compare(strings.ToLower(a.Command), strings.ToLower(b.Command))
		}
	default: // SortCPU
		return func(a, b model.Process) int { return cmpFloat(a.CPUPercent, b.CPUPercent) }
	}
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
