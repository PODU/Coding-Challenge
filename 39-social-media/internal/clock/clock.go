// Package clock lets tests control the time the scheduler sees.
package clock

import (
	"sync"
	"time"
)

// Clock returns the current time, in UTC.
type Clock interface {
	Now() time.Time
}

// System is the real clock.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

// Fake is a Clock that only moves when told to.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

func NewFake(t time.Time) *Fake { return &Fake{now: t.UTC()} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the fake clock forward by d and returns the new time.
func (f *Fake) Advance(d time.Duration) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d).UTC()
	return f.now
}

func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t.UTC()
}
