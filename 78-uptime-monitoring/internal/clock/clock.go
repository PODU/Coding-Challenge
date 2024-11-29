// Package clock lets the scheduler run on a fake clock in tests.
package clock

import (
	"sync"
	"time"
)

// Ticker abstracts time.Ticker so it can be faked.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// Clock is the injectable time source used throughout the service.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
	NewTicker(d time.Duration) Ticker
}

// RealClock is the production Clock backed by the stdlib time package.
type RealClock struct{}

// New returns a RealClock.
func New() RealClock { return RealClock{} }

func (RealClock) Now() time.Time                         { return time.Now() }
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (RealClock) NewTicker(d time.Duration) Ticker       { return &realTicker{t: time.NewTicker(d)} }

type realTicker struct{ t *time.Ticker }

func (r *realTicker) C() <-chan time.Time { return r.t.C }
func (r *realTicker) Stop()               { r.t.Stop() }

// FakeClock is a deterministic Clock for tests. Time only moves when Advance is
// called. Tickers use blocking sends so that Advance applies back-pressure: it
// will not fire the next tick until the consuming goroutine has received the
// current one. This makes scheduler tests race-free with no wall-clock sleeps.
//
// One-shot After channels are buffered so Advance never blocks on them.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*waiter
}

type waiter struct {
	until    time.Time
	ch       chan time.Time
	period   time.Duration // 0 => one-shot (After); >0 => ticker
	blocking bool          // ticker fires use blocking sends
}

// NewFake returns a FakeClock starting at the given time (or a fixed default).
func NewFake(start time.Time) *FakeClock {
	if start.IsZero() {
		start = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return &FakeClock{now: start}
}

// NumWaiters returns the number of registered timers/tickers. Tests use it to
// wait until the scheduler has installed its tickers before advancing time.
func (f *FakeClock) NumWaiters() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.waiters)
}

// Now returns the current virtual time.
func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// After returns a channel that receives once virtual time has advanced by d.
func (f *FakeClock) After(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- f.now
		return ch
	}
	f.waiters = append(f.waiters, &waiter{until: f.now.Add(d), ch: ch})
	return ch
}

// NewTicker returns a Ticker that fires every d of virtual time.
func (f *FakeClock) NewTicker(d time.Duration) Ticker {
	if d <= 0 {
		panic("clock: non-positive interval for NewTicker")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	w := &waiter{until: f.now.Add(d), ch: make(chan time.Time), period: d, blocking: true}
	f.waiters = append(f.waiters, w)
	return &fakeTicker{clock: f, w: w}
}

type fakeTicker struct {
	clock *FakeClock
	w     *waiter
}

func (t *fakeTicker) C() <-chan time.Time { return t.w.ch }
func (t *fakeTicker) Stop()               { t.clock.remove(t.w) }

func (f *FakeClock) remove(w *waiter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, x := range f.waiters {
		if x == w {
			f.waiters = append(f.waiters[:i], f.waiters[i+1:]...)
			return
		}
	}
}

// Advance moves virtual time forward by d, firing every waiter whose deadline is
// reached, in chronological order. Ticker fires use blocking sends (the lock is
// released around the send) so the caller is paced by the consumer.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	target := f.now.Add(d)
	for {
		next := f.earliest(target)
		if next == nil {
			f.now = target
			f.mu.Unlock()
			return
		}
		f.now = next.until
		ch := next.ch
		fireTime := next.until
		blocking := next.blocking
		if next.period > 0 {
			next.until = next.until.Add(next.period)
		} else {
			f.removeLocked(next)
		}
		if blocking {
			f.mu.Unlock()
			ch <- fireTime // paced by the consumer
			f.mu.Lock()
		} else {
			select {
			case ch <- fireTime:
			default:
			}
		}
	}
}

func (f *FakeClock) earliest(target time.Time) *waiter {
	// stable order so equal deadlines fire deterministically by registration
	var found *waiter
	for _, w := range f.waiters {
		if w.until.After(target) {
			continue
		}
		if found == nil || w.until.Before(found.until) {
			found = w
		}
	}
	return found
}

func (f *FakeClock) removeLocked(w *waiter) {
	for i, x := range f.waiters {
		if x == w {
			f.waiters = append(f.waiters[:i], f.waiters[i+1:]...)
			return
		}
	}
}
