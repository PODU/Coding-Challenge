package source

import (
	"context"
	"testing"
	"time"
)

// TestFakeContract asserts the fake source satisfies the ProcessSource contract:
// successive Sample calls advance wall time and per-process CPU counters
// monotonically, which is what the engine relies on to derive stable rates.
func TestFakeContract(t *testing.T) {
	var src ProcessSource = NewFake(time.Second)
	ctx := context.Background()

	s1, err := src.Sample(ctx)
	if err != nil {
		t.Fatalf("Sample 1: %v", err)
	}
	s2, err := src.Sample(ctx)
	if err != nil {
		t.Fatalf("Sample 2: %v", err)
	}

	if !s2.Time.After(s1.Time) {
		t.Errorf("time did not advance: %v !> %v", s2.Time, s1.Time)
	}
	if len(s1.Procs) == 0 || len(s2.Procs) != len(s1.Procs) {
		t.Fatalf("proc counts: %d then %d", len(s1.Procs), len(s2.Procs))
	}
	if s2.Mem.Total == 0 {
		t.Error("expected non-zero memory total")
	}

	prev := map[int]float64{}
	for _, p := range s1.Procs {
		prev[p.PID] = p.CPUTime
	}
	advanced := false
	for _, p := range s2.Procs {
		old, ok := prev[p.PID]
		if !ok {
			t.Errorf("PID %d missing from first sample", p.PID)
			continue
		}
		if p.CPUTime < old {
			t.Errorf("PID %d CPU counter went backwards: %v < %v", p.PID, p.CPUTime, old)
		}
		if p.CPUTime > old {
			advanced = true
		}
	}
	if !advanced {
		t.Error("expected at least one process CPU counter to advance")
	}

	if err := src.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
