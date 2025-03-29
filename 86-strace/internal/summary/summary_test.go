package summary

import (
	"strings"
	"testing"
	"time"
)

// build a deterministic aggregator:
//
//	openat: 3 calls, 30us total, 1 error
//	read:   2 calls, 10us total, 0 errors
//
// total: 5 calls, 40us, 1 error. openat = 75% time, read = 25%.
func sample() *Aggregator {
	a := New()
	a.Record("openat", 10*time.Microsecond, 2) // errno ENOENT
	a.Record("openat", 10*time.Microsecond, 0)
	a.Record("openat", 10*time.Microsecond, 0)
	a.Record("read", 5*time.Microsecond, 0)
	a.Record("read", 5*time.Microsecond, 0)
	return a
}

func TestRenderGolden(t *testing.T) {
	got := sample().Render()
	want := "" +
		"% time     seconds  usecs/call     calls    errors syscall\n" +
		"------ ----------- ----------- --------- --------- ----------------\n" +
		" 75.00    0.000030          10         3         1 openat\n" +
		" 25.00    0.000010           5         2           read\n" +
		"------ ----------- ----------- --------- --------- ----------------\n" +
		"100.00    0.000040                     5         1 total\n"
	if got != want {
		t.Errorf("Render mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderSortOrder(t *testing.T) {
	// openat (more time) must precede read.
	out := sample().Render()
	oi := strings.Index(out, "openat")
	ri := strings.Index(out, "read")
	if oi < 0 || ri < 0 || oi > ri {
		t.Errorf("expected openat before read, got:\n%s", out)
	}
}

func TestRenderTotalsAndPercent(t *testing.T) {
	out := sample().Render()
	if !strings.Contains(out, "         5         1 total") {
		t.Errorf("total row wrong:\n%s", out)
	}
	// % time of the listed syscalls must sum to 100 (the total row prints 100.00).
	if !strings.Contains(out, "100.00") {
		t.Errorf("total %% time should be 100.00:\n%s", out)
	}
}

func TestRecordCountsErrors(t *testing.T) {
	a := New()
	a.Record("close", time.Microsecond, 0)
	a.Record("close", time.Microsecond, 9) // EBADF
	s := a.stats["close"]
	if s.calls != 2 || s.errors != 1 {
		t.Errorf("close calls=%d errors=%d, want 2/1", s.calls, s.errors)
	}
}

func TestRenderEmpty(t *testing.T) {
	out := New().Render()
	if !strings.Contains(out, "total") {
		t.Errorf("empty render should still have a total row:\n%s", out)
	}
}
