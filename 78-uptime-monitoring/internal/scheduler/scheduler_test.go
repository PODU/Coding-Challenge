package scheduler

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/example/uptime/internal/clock"
	"github.com/example/uptime/internal/monitor"
	"github.com/example/uptime/internal/store"
)

// stubChecker counts invocations and can block until released, to exercise the
// overrun-skip and isolation behaviours deterministically.
type stubChecker struct {
	mu     sync.Mutex
	calls  map[string]int
	status monitor.Status
	block  map[string]chan struct{} // per-monitor gate; nil => no blocking
}

func newStub(status monitor.Status) *stubChecker {
	return &stubChecker{calls: map[string]int{}, status: status, block: map[string]chan struct{}{}}
}

func (s *stubChecker) Check(_ context.Context, m monitor.Monitor) monitor.CheckResult {
	s.mu.Lock()
	s.calls[m.ID]++
	gate := s.block[m.ID]
	s.mu.Unlock()
	if gate != nil {
		<-gate
	}
	return monitor.CheckResult{MonitorID: m.ID, Status: s.status, Timestamp: time.Now()}
}

func (s *stubChecker) count(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[id]
}

func waitForWaiters(fake *clock.FakeClock, n int) {
	for fake.NumWaiters() < n {
		runtime.Gosched()
	}
}

func newSched(fake *clock.FakeClock, c *stubChecker) (*Scheduler, chan monitor.CheckResult) {
	s := New(Options{Clock: fake, Checker: c, Store: store.NewMemory()})
	done := make(chan monitor.CheckResult, 64)
	s.SetAfterCheck(func(r monitor.CheckResult) { done <- r })
	return s, done
}

func httpMon(id string, interval time.Duration) monitor.Monitor {
	return monitor.Monitor{ID: id, Kind: monitor.KindHTTP, Target: "https://x/",
		Interval: interval, Timeout: interval / 2, Enabled: true}
}

func TestScheduler_TicksAtInterval(t *testing.T) {
	fake := clock.NewFake(time.Time{})
	stub := newStub(monitor.StatusUp)
	s, done := newSched(fake, stub)

	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx, []monitor.Monitor{httpMon("m", 30*time.Second)})
	waitForWaiters(fake, 1)

	for i := 0; i < 3; i++ {
		fake.Advance(30 * time.Second)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("check %d not invoked", i+1)
		}
	}
	cancel()
	if got := stub.count("m"); got != 3 {
		t.Fatalf("expected exactly 3 checks, got %d", got)
	}
}

func TestScheduler_IndependentIntervals(t *testing.T) {
	fake := clock.NewFake(time.Time{})
	stub := newStub(monitor.StatusUp)
	s, done := newSched(fake, stub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx, []monitor.Monitor{httpMon("fast", 10*time.Second), httpMon("slow", 30*time.Second)})
	waitForWaiters(fake, 2)

	// Advance 30s in 10s steps; fast should fire 3 times, slow once.
	for i := 0; i < 3; i++ {
		fake.Advance(10 * time.Second)
		// drain whatever fired this step
		drainFor(done, 200*time.Millisecond)
	}
	if got := stub.count("fast"); got != 3 {
		t.Fatalf("fast checks = %d, want 3", got)
	}
	if got := stub.count("slow"); got != 1 {
		t.Fatalf("slow checks = %d, want 1", got)
	}
}

func TestScheduler_SkipsOverrunTick(t *testing.T) {
	fake := clock.NewFake(time.Time{})
	stub := newStub(monitor.StatusDown)
	gate := make(chan struct{})
	stub.block["m"] = gate

	s, done := newSched(fake, stub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx, []monitor.Monitor{httpMon("m", 30*time.Second)})
	waitForWaiters(fake, 1)

	fake.Advance(30 * time.Second) // tick 1 -> check starts and blocks (in-flight)
	fake.Advance(30 * time.Second) // tick 2 -> must be skipped, not queued
	close(gate)                    // release check 1

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked check never completed")
	}
	// Give a brief window to prove tick 2 produced no second check.
	if extra := drainFor(done, 200*time.Millisecond); extra != 0 {
		t.Fatalf("expected no extra checks from skipped tick, got %d", extra)
	}
	if got := stub.count("m"); got != 1 {
		t.Fatalf("expected exactly 1 check (overrun skipped), got %d", got)
	}
	if skipped := s.metrics.TicksSkipped.Load(); skipped != 1 {
		t.Fatalf("expected 1 skipped tick recorded, got %d", skipped)
	}
}

func TestScheduler_SlowMonitorDoesNotBlockOthers(t *testing.T) {
	fake := clock.NewFake(time.Time{})
	stub := newStub(monitor.StatusUp)
	gate := make(chan struct{})
	stub.block["slow"] = gate // slow monitor blocks indefinitely

	s, done := newSched(fake, stub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx, []monitor.Monitor{httpMon("slow", 30*time.Second), httpMon("fast", 30*time.Second)})
	waitForWaiters(fake, 2)

	fake.Advance(30 * time.Second) // both tick; slow blocks, fast should still complete

	select {
	case r := <-done:
		if r.MonitorID != "fast" {
			t.Fatalf("expected fast monitor to complete first, got %q", r.MonitorID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fast monitor was starved by the slow one")
	}
	close(gate)
}

func TestScheduler_GracefulDrainOnCancel(t *testing.T) {
	fake := clock.NewFake(time.Time{})
	stub := newStub(monitor.StatusUp)
	s, _ := newSched(fake, stub)

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() { s.Run(ctx, []monitor.Monitor{httpMon("m", 30*time.Second)}); close(finished) }()
	waitForWaiters(fake, 1)

	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel (loops leaked)")
	}
}

// drainFor consumes results for up to d and returns how many it saw.
func drainFor(ch <-chan monitor.CheckResult, d time.Duration) int {
	timer := time.NewTimer(d)
	defer timer.Stop()
	n := 0
	for {
		select {
		case <-ch:
			n++
		case <-timer.C:
			return n
		}
	}
}
