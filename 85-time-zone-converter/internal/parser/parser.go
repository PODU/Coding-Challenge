// Package parser reads the time the user typed as a wall-clock time in the
// source zone, noticing when it falls in a DST gap or overlap.
package parser

import (
	"fmt"
	"strings"
	"time"
)

type Result struct {
	Time      time.Time // in the source zone
	Layout    string    // the layout that matched, or "now" or "rfc3339"
	Input     string
	Gap       bool // the wall time was skipped by a spring-forward change
	Ambiguous bool // the wall time happens twice because of a fall-back change
}

// AcceptedLayouts is the list of formats shown to the user in errors.
var AcceptedLayouts = []string{
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"15:04",
	"3:04PM",
	"RFC3339 (e.g. 2006-01-02T15:04:05Z07:00)",
	"now",
}

// dateTimeLayouts are parsed in UTC just to get the date and time fields,
// which are then placed in the source zone.
var dateTimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// timeOnlyLayouts use today's date in the source zone.
var timeOnlyLayouts = []string{
	"15:04:05",
	"15:04",
	"3:04PM",
	"3:04 PM",
}

type ParseError struct{ Input string }

func (e *ParseError) Error() string {
	return fmt.Sprintf("could not parse time %q; accepted formats: %s",
		e.Input, strings.Join(AcceptedLayouts, ", "))
}

// Parse reads input as a time in loc. An empty string or "now" gives now;
// now also supplies the date when only a time is given.
func Parse(input string, loc *time.Location, now time.Time) (Result, error) {
	s := strings.TrimSpace(input)
	if s == "" || strings.EqualFold(s, "now") {
		return Result{Time: now.In(loc), Layout: "now", Input: input}, nil
	}

	// RFC 3339 carries its own offset, so it names an exact instant.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return Result{Time: t.In(loc), Layout: "rfc3339", Input: input}, nil
	}

	for _, layout := range dateTimeLayouts {
		if tu, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return build(tu.Year(), tu.Month(), tu.Day(), tu.Hour(), tu.Minute(), tu.Second(), tu.Nanosecond(), loc, layout, input), nil
		}
	}

	for _, layout := range timeOnlyLayouts {
		if tu, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			n := now.In(loc)
			return build(n.Year(), n.Month(), n.Day(), tu.Hour(), tu.Minute(), tu.Second(), 0, loc, layout, input), nil
		}
	}

	return Result{}, &ParseError{Input: input}
}

// build makes the time in loc and works out whether it hit a DST gap or overlap.
func build(y int, mo time.Month, d, h, mi, se, ns int, loc *time.Location, layout, input string) Result {
	t := time.Date(y, mo, d, h, mi, se, ns, loc)

	// In a spring-forward gap the requested time doesn't exist. time.Date then
	// picks an offset without saying which, and the clock reads back different.
	gap := t.Hour() != h || t.Minute() != mi || t.Day() != d || t.Month() != mo

	ambiguous := false
	if gap {
		// Use the offset from before the change (the one at midnight), which
		// moves the time forward: 02:30 becomes 03:30 in the US, as most
		// software does.
		dayStart := time.Date(y, mo, d, 0, 0, 0, 0, loc)
		_, preOff := dayStart.Zone()
		wUTC := time.Date(y, mo, d, h, mi, se, ns, time.UTC)
		t = wUTC.Add(-time.Duration(preOff) * time.Second).In(loc)
	} else {
		// In a fall-back overlap the same clock time happens twice and Go picks
		// the first. If moving forward by a DST-sized step gives the same clock
		// time with another offset, we're in the overlap.
		_, off0 := t.Zone()
		for _, delta := range []time.Duration{30 * time.Minute, time.Hour} {
			t2 := t.Add(delta)
			if t2.Hour() == h && t2.Minute() == mi && t2.Day() == d {
				if _, off2 := t2.Zone(); off2 != off0 {
					ambiguous = true
				}
			}
		}
	}

	return Result{Time: t, Layout: layout, Input: input, Gap: gap, Ambiguous: ambiguous}
}
