package cron

import (
	"testing"
	"time"

	_ "time/tzdata" // embed zoneinfo so DST tests run anywhere
)

func mustParse(t *testing.T, expr string) *Schedule {
	t.Helper()
	s, err := Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q): %v", expr, err)
	}
	return s
}

func rfc(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad time %q: %v", s, err)
	}
	return tm
}

func TestNextRun(t *testing.T) {
	tests := []struct {
		name string
		expr string
		from string
		want string
	}{
		{"every minute", "* * * * *", "2026-06-26T08:50:30Z", "2026-06-26T08:51:00Z"},
		{"step 15", "*/15 9-17 * * 1-5", "2026-06-26T08:50:00Z", "2026-06-26T09:00:00Z"},
		{"hourly", "0 * * * *", "2026-06-26T08:50:00Z", "2026-06-26T09:00:00Z"},
		{"daily next day", "0 9 * * *", "2026-06-26T09:00:00Z", "2026-06-27T09:00:00Z"},
		{"weekday skips weekend", "0 9 * * 1-5", "2026-06-26T09:30:00Z", "2026-06-29T09:00:00Z"},
		{"named saturday", "0 9 * * SAT", "2026-06-26T10:00:00Z", "2026-06-27T09:00:00Z"},
		{"month rollover", "0 0 1 * *", "2026-06-15T00:00:00Z", "2026-07-01T00:00:00Z"},
		{"year rollover", "0 0 1 1 *", "2026-06-15T00:00:00Z", "2027-01-01T00:00:00Z"},
		{"leap year feb29", "0 0 29 2 *", "2025-03-01T00:00:00Z", "2028-02-29T00:00:00Z"},
		{"31st skips short month", "0 0 31 * *", "2026-04-01T00:00:00Z", "2026-05-31T00:00:00Z"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mustParse(t, tc.expr).NextRun(rfc(t, tc.from))
			if err != nil {
				t.Fatalf("NextRun: %v", err)
			}
			want := rfc(t, tc.want)
			if !got.Equal(want) {
				t.Errorf("NextRun(%q, %q) = %s, want %s", tc.expr, tc.from, got.Format(time.RFC3339), tc.want)
			}
		})
	}
}

// TestNextRunDomDowOR verifies the OR semantics: "0 0 13 * FRI" fires on every
// Friday OR the 13th of any month.
func TestNextRunDomDowOR(t *testing.T) {
	s := mustParse(t, "0 0 13 * FRI")
	from := rfc(t, "2026-06-26T00:00:00Z") // a Friday, at midnight
	runs, err := s.NextRuns(from, 3)
	if err != nil {
		t.Fatalf("NextRuns: %v", err)
	}
	want := []string{
		"2026-07-03T00:00:00Z", // Friday (not the 13th)
		"2026-07-10T00:00:00Z", // Friday (not the 13th)
		"2026-07-13T00:00:00Z", // the 13th (a Monday) -> proves DOM-only match
	}
	if len(runs) != len(want) {
		t.Fatalf("got %d runs, want %d", len(runs), len(want))
	}
	for i, w := range want {
		if !runs[i].Equal(rfc(t, w)) {
			t.Errorf("run[%d] = %s, want %s", i, runs[i].Format(time.RFC3339), w)
		}
	}
}

func TestNextRunsStrictlyIncreasing(t *testing.T) {
	s := mustParse(t, "*/15 * * * *")
	runs, err := s.NextRuns(rfc(t, "2026-06-26T08:50:00Z"), 5)
	if err != nil {
		t.Fatalf("NextRuns: %v", err)
	}
	if len(runs) != 5 {
		t.Fatalf("got %d runs, want 5", len(runs))
	}
	for i := 1; i < len(runs); i++ {
		if !runs[i].After(runs[i-1]) {
			t.Errorf("runs not strictly increasing at %d: %s !> %s", i, runs[i], runs[i-1])
		}
	}
}

func TestNextRunImpossible(t *testing.T) {
	s := mustParse(t, "0 0 30 2 *") // Feb never has 30 days
	_, err := s.NextRun(rfc(t, "2026-01-01T00:00:00Z"))
	if err == nil {
		t.Fatal("expected error for impossible schedule, got nil")
	}
}

func TestNextRunDSTSpringForward(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	// 02:30 on 2026-03-08 does not exist (spring forward), so the job does not
	// fire that year; the next valid 02:30 on a March 8 is 2027.
	s := mustParse(t, "30 2 8 3 *")
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, ny)
	got, err := s.NextRun(from)
	if err != nil {
		t.Fatalf("NextRun: %v", err)
	}
	if got.Year() != 2027 || got.Month() != 3 || got.Day() != 8 || got.Hour() != 2 || got.Minute() != 30 {
		t.Errorf("spring-forward next = %s, want 2027-03-08 02:30 local", got)
	}
}

func TestNextRunDSTFallBack(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	// 01:30 on 2026-11-01 occurs twice (fall back). NextRun returns the first
	// (EDT) occurrence.
	s := mustParse(t, "30 1 1 11 *")
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, ny)
	got, err := s.NextRun(from)
	if err != nil {
		t.Fatalf("NextRun: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 11 || got.Day() != 1 || got.Hour() != 1 || got.Minute() != 30 {
		t.Errorf("fall-back next = %s, want 2026-11-01 01:30 local", got)
	}
	_, offset := got.Zone()
	if offset != -4*3600 {
		t.Errorf("fall-back offset = %d, want -14400 (EDT, first occurrence)", offset)
	}
}

func TestMatches(t *testing.T) {
	s := mustParse(t, "0 9 * * SAT")
	if !s.Matches(rfc(t, "2026-06-27T09:00:00Z")) { // Saturday 09:00
		t.Error("expected match on Saturday 09:00")
	}
	if s.Matches(rfc(t, "2026-06-26T09:00:00Z")) { // Friday 09:00
		t.Error("unexpected match on Friday 09:00")
	}
}
