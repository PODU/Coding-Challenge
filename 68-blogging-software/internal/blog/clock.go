package blog

import "time"

// Clock returns the current time. Tests use FixedClock.
type Clock func() time.Time

// SystemClock returns the current time in UTC.
func SystemClock() time.Time { return time.Now().UTC() }

// FixedClock always returns t.
func FixedClock(t time.Time) Clock {
	return func() time.Time { return t }
}
