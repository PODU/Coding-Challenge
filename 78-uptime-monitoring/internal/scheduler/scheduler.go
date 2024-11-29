// Package scheduler probes each monitor on its own interval, stores the
// results and passes them to the alert tracker. A slow target doesn't delay
// the others; if a check is still running when its next tick comes, that tick
// is skipped.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/example/uptime/internal/alert"
	"github.com/example/uptime/internal/checker"
	"github.com/example/uptime/internal/clock"
	"github.com/example/uptime/internal/monitor"
	"github.com/example/uptime/internal/store"
)

// Metrics holds process-wide counters surfaced on /metrics.
type Metrics struct {
	ChecksTotal  atomic.Int64
	ChecksUp     atomic.Int64
	ChecksDown   atomic.Int64
	SaveErrors   atomic.Int64
	TicksSkipped atomic.Int64
	AlertsFired  atomic.Int64
}

// Options configures a Scheduler.
type Options struct {
	Clock       clock.Clock
	Checker     checker.Checker
	Store       store.Store
	Tracker     *alert.StateTracker
	Alerter     alert.Alerter
	Logger      *slog.Logger
	Metrics     *Metrics
	MaxInFlight int // global cap on concurrent checks (default 16)
}

// Scheduler runs the probe loops for a set of monitors.
type Scheduler struct {
	clk     clock.Clock
	checker checker.Checker
	store   store.Store
	tracker *alert.StateTracker
	alerter alert.Alerter
	log     *slog.Logger
	metrics *Metrics

	sem chan struct{}
	wg  sync.WaitGroup

	// afterCheck, if set, is invoked after each check completes and its
	// in-flight flag is cleared. Used by tests to synchronise with the clock.
	afterCheck func(monitor.CheckResult)
}

// New builds a Scheduler from Options, applying defaults.
func New(o Options) *Scheduler {
	max := o.MaxInFlight
	if max <= 0 {
		max = 16
	}
	log := o.Logger
	if log == nil {
		log = slog.Default()
	}
	m := o.Metrics
	if m == nil {
		m = &Metrics{}
	}
	return &Scheduler{
		clk:     o.Clock,
		checker: o.Checker,
		store:   o.Store,
		tracker: o.Tracker,
		alerter: o.Alerter,
		log:     log,
		metrics: m,
		sem:     make(chan struct{}, max),
	}
}

// SetAfterCheck installs a test hook fired after every completed check.
func (s *Scheduler) SetAfterCheck(fn func(monitor.CheckResult)) { s.afterCheck = fn }

// Run starts a probe loop per enabled monitor and blocks until ctx is cancelled
// and all in-flight checks have drained.
func (s *Scheduler) Run(ctx context.Context, monitors []monitor.Monitor) {
	for _, m := range monitors {
		if !m.Enabled {
			continue
		}
		s.wg.Add(1)
		go s.runMonitor(ctx, m)
	}
	s.wg.Wait()
}

func (s *Scheduler) runMonitor(ctx context.Context, m monitor.Monitor) {
	defer s.wg.Done()
	ticker := s.clk.NewTicker(m.Interval)
	defer ticker.Stop()

	var inFlight atomic.Bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			if !inFlight.CompareAndSwap(false, true) {
				// Previous check still running: drop this tick.
				s.metrics.TicksSkipped.Add(1)
				s.log.Warn("skipping tick; previous check still in flight", "monitor", m.ID)
				continue
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer inFlight.Store(false)
				if !s.acquire(ctx) {
					return
				}
				defer s.release()
				r := s.runCheck(ctx, m)
				// Clear the in-flight flag before notifying the hook so that a
				// test waiting on the hook can safely advance the clock again.
				inFlight.Store(false)
				if s.afterCheck != nil {
					s.afterCheck(r)
				}
			}()
		}
	}
}

func (s *Scheduler) acquire(ctx context.Context) bool {
	select {
	case s.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Scheduler) release() { <-s.sem }

func (s *Scheduler) runCheck(ctx context.Context, m monitor.Monitor) monitor.CheckResult {
	cctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	r := s.checker.Check(cctx, m)

	s.metrics.ChecksTotal.Add(1)
	switch r.Status {
	case monitor.StatusUp:
		s.metrics.ChecksUp.Add(1)
	case monitor.StatusDown:
		s.metrics.ChecksDown.Add(1)
	}

	// Persist using the root ctx, not the per-check (already-deadlined) ctx, so
	// a store write isn't cut short by the probe timeout. A write failure must
	// not stop monitoring.
	if err := s.store.SaveResult(ctx, r); err != nil {
		s.metrics.SaveErrors.Add(1)
		s.log.Error("failed to persist result", "monitor", m.ID, "err", err)
	}

	if s.tracker != nil {
		if ev, fire := s.tracker.Observe(r); fire {
			s.metrics.AlertsFired.Add(1)
			if s.alerter != nil {
				if err := s.alerter.Notify(ctx, ev); err != nil {
					s.log.Error("alert delivery failed", "monitor", m.ID, "err", err)
				}
			}
		}
	}
	return r
}
