// Package clock lets the scheduler run against a fake clock in tests.
package clock

import (
	"context"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	// Sleep waits for d, returning ctx.Err() early if ctx is cancelled.
	Sleep(ctx context.Context, d time.Duration) error
}

// Real is the wall clock, reporting times in a fixed location.
type Real struct {
	loc *time.Location
}

// NewReal returns a Real clock for loc, or time.Local if loc is nil.
func NewReal(loc *time.Location) *Real {
	if loc == nil {
		loc = time.Local
	}
	return &Real{loc: loc}
}

func (r *Real) Now() time.Time { return time.Now().In(r.loc) }

func (r *Real) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Fake is a test Clock. Time only moves when Advance is called.
type Fake struct {
	mu      sync.Mutex
	cond    *sync.Cond
	now     time.Time
	waiters []*fakeWaiter
}

type fakeWaiter struct {
	wake time.Time
	ch   chan struct{}
}

// NewFake returns a Fake clock positioned at start.
func NewFake(start time.Time) *Fake {
	f := &Fake{now: start}
	f.cond = sync.NewCond(&f.mu)
	return f
}

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	f.mu.Lock()
	w := &fakeWaiter{wake: f.now.Add(d), ch: make(chan struct{})}
	f.waiters = append(f.waiters, w)
	f.cond.Broadcast() // for BlockUntil
	f.mu.Unlock()

	select {
	case <-ctx.Done():
		f.remove(w)
		return ctx.Err()
	case <-w.ch:
		return nil
	}
}

// Advance moves time forward by d and wakes the sleepers that are now due.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	var remaining []*fakeWaiter
	for _, w := range f.waiters {
		if !w.wake.After(f.now) {
			close(w.ch)
		} else {
			remaining = append(remaining, w)
		}
	}
	f.waiters = remaining
	f.cond.Broadcast()
	f.mu.Unlock()
}

// BlockUntil waits until at least n goroutines are in Sleep, so a test can be
// sure the scheduler is waiting before it calls Advance.
func (f *Fake) BlockUntil(n int) {
	f.mu.Lock()
	for len(f.waiters) < n {
		f.cond.Wait()
	}
	f.mu.Unlock()
}

func (f *Fake) remove(target *fakeWaiter) {
	f.mu.Lock()
	for i, w := range f.waiters {
		if w == target {
			f.waiters = append(f.waiters[:i], f.waiters[i+1:]...)
			break
		}
	}
	f.cond.Broadcast()
	f.mu.Unlock()
}
