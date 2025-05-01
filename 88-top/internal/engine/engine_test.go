package engine

import (
	"math"
	"testing"
	"time"

	"github.com/example/cctop/internal/model"
	"github.com/example/cctop/internal/source"
)

var epoch = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

// sample builds a source.Sample at epoch+offset with the given procs.
func sample(offset time.Duration, total uint64, host source.CPUTimes, procs ...source.ProcSample) source.Sample {
	return source.Sample{
		Time:    epoch.Add(offset),
		NumCPU:  4,
		Mem:     source.MemStat{Total: total},
		HostCPU: host,
		Procs:   procs,
	}
}

func proc(pid int, cpuTime float64, rss uint64) source.ProcSample {
	return source.ProcSample{PID: pid, User: "u", Command: "c", State: "S", CPUTime: cpuTime, RSS: rss}
}

func findProc(snap model.Snapshot, pid int) (model.Process, bool) {
	for _, p := range snap.Procs {
		if p.PID == pid {
			return p, true
		}
	}
	return model.Process{}, false
}

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestCPUDelta(t *testing.T) {
	view := model.ViewState{Sort: model.SortPID}

	t.Run("0.5s over 1s = 50%", func(t *testing.T) {
		prev := sample(0, 1<<30, source.CPUTimes{}, proc(100, 0.0, 0))
		cur := sample(time.Second, 1<<30, source.CPUTimes{}, proc(100, 0.5, 0))
		snap := Compute(prev, cur, view)
		p, _ := findProc(snap, 100)
		approx(t, "cpu%", p.CPUPercent, 50.0)
	})

	t.Run("0.2s over 2s = 10%", func(t *testing.T) {
		prev := sample(0, 1<<30, source.CPUTimes{}, proc(100, 0.0, 0))
		cur := sample(2*time.Second, 1<<30, source.CPUTimes{}, proc(100, 0.2, 0))
		snap := Compute(prev, cur, view)
		p, _ := findProc(snap, 100)
		approx(t, "cpu%", p.CPUPercent, 10.0)
	})

	t.Run("solaris divides by NumCPU", func(t *testing.T) {
		sview := view
		sview.Solaris = true
		prev := sample(0, 1<<30, source.CPUTimes{}, proc(100, 0.0, 0))
		cur := sample(time.Second, 1<<30, source.CPUTimes{}, proc(100, 0.5, 0))
		snap := Compute(prev, cur, sview)
		p, _ := findProc(snap, 100)
		approx(t, "cpu%", p.CPUPercent, 12.5) // 50 / 4
	})

	t.Run("new pid -> 0", func(t *testing.T) {
		prev := sample(0, 1<<30, source.CPUTimes{}, proc(100, 0.0, 0))
		cur := sample(time.Second, 1<<30, source.CPUTimes{}, proc(100, 0.5, 0), proc(200, 9.9, 0))
		snap := Compute(prev, cur, view)
		p, _ := findProc(snap, 200)
		approx(t, "cpu%", p.CPUPercent, 0.0)
	})

	t.Run("vanished pid dropped", func(t *testing.T) {
		prev := sample(0, 1<<30, source.CPUTimes{}, proc(100, 0.0, 0), proc(999, 1.0, 0))
		cur := sample(time.Second, 1<<30, source.CPUTimes{}, proc(100, 0.5, 0))
		snap := Compute(prev, cur, view)
		if _, ok := findProc(snap, 999); ok {
			t.Error("vanished PID 999 should be absent")
		}
	})

	t.Run("deltaWall 0 -> 0, no panic", func(t *testing.T) {
		prev := sample(0, 1<<30, source.CPUTimes{}, proc(100, 0.0, 0))
		cur := sample(0, 1<<30, source.CPUTimes{}, proc(100, 0.5, 0))
		snap := Compute(prev, cur, view)
		p, _ := findProc(snap, 100)
		approx(t, "cpu%", p.CPUPercent, 0.0)
	})

	t.Run("counter rollback clamped", func(t *testing.T) {
		prev := sample(0, 1<<30, source.CPUTimes{}, proc(100, 5.0, 0))
		cur := sample(time.Second, 1<<30, source.CPUTimes{}, proc(100, 3.0, 0))
		snap := Compute(prev, cur, view)
		p, _ := findProc(snap, 100)
		approx(t, "cpu%", p.CPUPercent, 0.0)
	})

	t.Run("first frame (no prev) -> 0", func(t *testing.T) {
		var prev source.Sample // zero Time
		cur := sample(time.Second, 1<<30, source.CPUTimes{}, proc(100, 0.5, 0))
		snap := Compute(prev, cur, view)
		p, _ := findProc(snap, 100)
		approx(t, "cpu%", p.CPUPercent, 0.0)
	})
}

func TestMemPercent(t *testing.T) {
	view := model.ViewState{Sort: model.SortPID}
	const total = uint64(4) << 30 // 4 GiB
	const rss = uint64(1) << 30   // 1 GiB
	prev := sample(0, total, source.CPUTimes{}, proc(100, 0, rss))
	cur := sample(time.Second, total, source.CPUTimes{}, proc(100, 0, rss))
	snap := Compute(prev, cur, view)
	p, _ := findProc(snap, 100)
	approx(t, "mem%", p.MemPercent, 25.0)
}

func TestHostCPUAggregate(t *testing.T) {
	view := model.ViewState{Sort: model.SortPID}
	prevHost := source.CPUTimes{}
	curHost := source.CPUTimes{User: 10, System: 5, Idle: 85}
	prev := sample(0, 1<<30, prevHost, proc(100, 0, 0))
	cur := sample(time.Second, 1<<30, curHost, proc(100, 0, 0))
	snap := Compute(prev, cur, view)
	approx(t, "us", snap.Summary.CPUUser, 10.0)
	approx(t, "sy", snap.Summary.CPUSys, 5.0)
	approx(t, "id", snap.Summary.CPUIdle, 85.0)
}

func TestSummaryTaskCounts(t *testing.T) {
	view := model.ViewState{Sort: model.SortPID}
	procs := []source.ProcSample{
		{PID: 1, State: "R"},
		{PID: 2, State: "S"},
		{PID: 3, State: "sleep"},
		{PID: 4, State: "running"},
		{PID: 5, State: "T"},
		{PID: 6, State: "Z"},
		{PID: 7, State: ""}, // unknown -> sleeping
	}
	cur := sample(time.Second, 1<<30, source.CPUTimes{}, procs...)
	prev := sample(0, 1<<30, source.CPUTimes{})
	snap := Compute(prev, cur, view)
	s := snap.Summary
	if s.TasksTotal != 7 {
		t.Errorf("total = %d, want 7", s.TasksTotal)
	}
	if s.Running != 2 {
		t.Errorf("running = %d, want 2", s.Running)
	}
	if s.Stopped != 1 {
		t.Errorf("stopped = %d, want 1", s.Stopped)
	}
	if s.Zombie != 1 {
		t.Errorf("zombie = %d, want 1", s.Zombie)
	}
	if s.Sleeping != 3 {
		t.Errorf("sleeping = %d, want 3", s.Sleeping)
	}
}

func TestUserFilter(t *testing.T) {
	view := model.ViewState{Sort: model.SortPID, UserFilter: "alice"}
	cur := source.Sample{
		Time:   epoch.Add(time.Second),
		NumCPU: 4,
		Mem:    source.MemStat{Total: 1 << 30},
		Procs: []source.ProcSample{
			{PID: 1, User: "alice"},
			{PID: 2, User: "bob"},
			{PID: 3, User: "alice"},
		},
	}
	prev := sample(0, 1<<30, source.CPUTimes{})
	snap := Compute(prev, cur, view)
	if len(snap.Procs) != 2 {
		t.Fatalf("filtered procs = %d, want 2", len(snap.Procs))
	}
	for _, p := range snap.Procs {
		if p.User != "alice" {
			t.Errorf("unexpected user %q in filtered output", p.User)
		}
	}
}
