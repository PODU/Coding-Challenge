// Package report renders benchmark results for a terminal.
package report

import (
	"fmt"
	"strconv"
)

// Unit is a time unit used for display. Timings are always stored in seconds;
// a Unit only affects presentation.
type Unit int

// The supported display units.
const (
	Second Unit = iota
	Millisecond
	Microsecond
)

// UnitFor picks the unit that keeps a value readable: whole seconds stay
// seconds, sub-second values become milliseconds, and anything under a
// millisecond becomes microseconds.
func UnitFor(seconds float64) Unit {
	switch {
	case seconds < 1e-3:
		return Microsecond
	case seconds < 1:
		return Millisecond
	default:
		return Second
	}
}

// ParseUnit maps a `--time-unit` value onto a Unit. An empty string means
// "choose automatically" and is reported as ok == false.
func ParseUnit(s string) (Unit, bool) {
	switch s {
	case "second":
		return Second, true
	case "millisecond":
		return Millisecond, true
	case "microsecond":
		return Microsecond, true
	}
	return Second, false
}

// Short is the suffix printed next to a value.
func (u Unit) Short() string {
	switch u {
	case Millisecond:
		return "ms"
	case Microsecond:
		return "µs"
	default:
		return "s"
	}
}

// ASCII is the suffix used when the terminal cannot be trusted with unicode.
func (u Unit) ASCII() string {
	if u == Microsecond {
		return "us"
	}
	return u.Short()
}

// Scale converts a duration in seconds into this unit.
func (u Unit) Scale(seconds float64) float64 {
	switch u {
	case Millisecond:
		return seconds * 1e3
	case Microsecond:
		return seconds * 1e6
	default:
		return seconds
	}
}

// Format renders a duration in this unit. Seconds get three decimals because
// the interesting variation is in the milliseconds; smaller units get one,
// because further digits are below the timer's own resolution.
func (u Unit) Format(seconds float64) string {
	digits := 1
	if u == Second {
		digits = 3
	}
	return strconv.FormatFloat(u.Scale(seconds), 'f', digits, 64)
}

// Pad right-aligns s in a field of n characters. A value too wide for the field
// is never truncated; it keeps one leading space so it cannot run into the
// label printed to its left.
func Pad(s string, n int) string {
	if len(s) >= n {
		return " " + s
	}
	return fmt.Sprintf("%*s", n, s)
}
