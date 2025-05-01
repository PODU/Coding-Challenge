package engine

import (
	"testing"

	"github.com/example/cctop/internal/model"
)

func fixture() []model.Process {
	return []model.Process{
		{PID: 3, CPUPercent: 10, MemPercent: 5, CPUTime: 100, User: "bob", Command: "zsh"},
		{PID: 1, CPUPercent: 50, MemPercent: 1, CPUTime: 10, User: "alice", Command: "bash"},
		{PID: 2, CPUPercent: 10, MemPercent: 20, CPUTime: 50, User: "carol", Command: "emacs"},
	}
}

func order(procs []model.Process) []int {
	out := make([]int, len(procs))
	for i, p := range procs {
		out[i] = p.PID
	}
	return out
}

func eq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortOrders(t *testing.T) {
	cases := []struct {
		name    string
		key     model.SortKey
		reverse bool
		want    []int
	}{
		{"cpu desc (default)", model.SortCPU, false, []int{1, 2, 3}},
		{"cpu asc (reverse)", model.SortCPU, true, []int{2, 3, 1}},
		{"mem desc", model.SortMem, false, []int{2, 3, 1}},
		{"mem asc", model.SortMem, true, []int{1, 3, 2}},
		{"pid asc (default)", model.SortPID, false, []int{1, 2, 3}},
		{"pid desc (reverse)", model.SortPID, true, []int{3, 2, 1}},
		{"time desc", model.SortTime, false, []int{3, 2, 1}},
		{"user asc (default)", model.SortUser, false, []int{1, 3, 2}},
		{"command asc (default)", model.SortCommand, false, []int{1, 2, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			procs := fixture()
			sortProcs(procs, model.ViewState{Sort: c.key, Reverse: c.reverse})
			if got := order(procs); !eq(got, c.want) {
				t.Errorf("order = %v, want %v", got, c.want)
			}
		})
	}
}

// TestSortTieBreak verifies equal primary keys fall back to ascending PID.
func TestSortTieBreak(t *testing.T) {
	procs := []model.Process{
		{PID: 5, CPUPercent: 10},
		{PID: 2, CPUPercent: 10},
		{PID: 8, CPUPercent: 10},
	}
	sortProcs(procs, model.ViewState{Sort: model.SortCPU})
	if got := order(procs); !eq(got, []int{2, 5, 8}) {
		t.Errorf("tie-break order = %v, want [2 5 8]", got)
	}
}
