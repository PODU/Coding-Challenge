package source

import (
	"context"
	"testing"
)

// TestGopsutilSmoke is a light, host-dependent sanity check on the live source.
// It is skipped under -short. It does NOT assert correctness values (those are
// covered deterministically via the fake source); it only confirms the live
// collector returns plausible data on the build host.
func TestGopsutilSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live gopsutil smoke test in -short mode")
	}
	src := NewGopsutil()
	defer src.Close()

	s, err := src.Sample(context.Background())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if len(s.Procs) < 1 {
		t.Errorf("expected >= 1 process, got %d", len(s.Procs))
	}
	if s.Mem.Total == 0 {
		t.Error("expected non-zero memory total on host")
	}
	if s.NumCPU < 1 {
		t.Errorf("expected NumCPU >= 1, got %d", s.NumCPU)
	}
}
