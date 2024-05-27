package model

import (
	"testing"
	"time"
)

func TestDateArithmetic(t *testing.T) {
	d := Date{2026, time.March, 1}
	if got := d.AddDays(31); got != (Date{2026, time.April, 1}) {
		t.Errorf("AddDays(31) = %s, want 2026-04-01", got)
	}
	if got := d.AddDays(-1); got != (Date{2026, time.February, 28}) {
		t.Errorf("AddDays(-1) = %s, want 2026-02-28", got)
	}
	if n := (Date{2026, time.March, 11}).DaysSince(d); n != 10 {
		t.Errorf("DaysSince = %d, want 10", n)
	}
}

func TestDateComparisons(t *testing.T) {
	a := Date{2026, time.March, 1}
	b := Date{2026, time.March, 2}
	if !a.Before(b) || a.After(b) || !a.Equal(a) {
		t.Error("comparison ops wrong")
	}
	if a.Weekday() != time.Sunday {
		t.Errorf("2026-03-01 weekday = %s, want Sunday", a.Weekday())
	}
}

func TestDateOfAndIn(t *testing.T) {
	tm := time.Date(2026, 3, 1, 23, 30, 0, 0, time.FixedZone("x", -5*3600))
	if got := DateOf(tm); got != (Date{2026, time.March, 1}) {
		t.Errorf("DateOf = %s", got)
	}
	if got := DateIn(tm, time.UTC); got != (Date{2026, time.March, 2}) {
		t.Errorf("DateIn UTC = %s, want 2026-03-02", got)
	}
}

func TestParseDate(t *testing.T) {
	d, err := ParseDate("2026-03-15")
	if err != nil || d != (Date{2026, time.March, 15}) {
		t.Errorf("ParseDate = %s, %v", d, err)
	}
	if _, err := ParseDate("nope"); err == nil {
		t.Error("expected error for bad date")
	}
}

func TestDateString(t *testing.T) {
	if s := (Date{2026, time.March, 5}).String(); s != "2026-03-05" {
		t.Errorf("String = %q", s)
	}
}
