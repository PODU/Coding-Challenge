package parser

import (
	"errors"
	"testing"
	"time"

	_ "time/tzdata"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

// T7: US spring-forward gap. 02:30 on 2025-03-09 does not exist; the resulting
// instant equals 03:30 EDT and Gap is flagged.
func TestSpringForwardGap(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	res, err := Parse("2025-03-09 02:30", ny, time.Now())
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !res.Gap {
		t.Errorf("expected Gap=true for 02:30 spring-forward")
	}
	if got := res.Time.Format("15:04"); got != "03:30" {
		t.Errorf("wall time = %q, want 03:30", got)
	}
	abbr, off := res.Time.Zone()
	if abbr != "EDT" || off != -4*3600 {
		t.Errorf("zone = %s %d, want EDT -14400", abbr, off)
	}
}

// T8: US fall-back overlap. 01:30 on 2025-11-02 occurs twice; Go picks the
// earlier (EDT, -04:00) offset and Ambiguous is flagged.
func TestFallBackOverlap(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	res, err := Parse("2025-11-02 01:30", ny, time.Now())
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !res.Ambiguous {
		t.Errorf("expected Ambiguous=true for 01:30 fall-back")
	}
	if res.Gap {
		t.Errorf("fall-back should not be a gap")
	}
	_, off := res.Time.Zone()
	if off != -4*3600 {
		t.Errorf("offset = %d, want -14400 (earlier EDT)", off)
	}
}

// T15: each accepted layout parses to the expected instant in the source zone.
func TestLayouts(t *testing.T) {
	utc := time.UTC
	now := time.Date(2025, 6, 1, 8, 0, 0, 0, utc)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2025-06-01 09:30", time.Date(2025, 6, 1, 9, 30, 0, 0, utc)},
		{"2025-06-01T09:30", time.Date(2025, 6, 1, 9, 30, 0, 0, utc)},
		{"2025-06-01 09:30:15", time.Date(2025, 6, 1, 9, 30, 15, 0, utc)},
		{"2025-06-01", time.Date(2025, 6, 1, 0, 0, 0, 0, utc)},
		{"09:30", time.Date(2025, 6, 1, 9, 30, 0, 0, utc)}, // today's date from now
		{"9:30AM", time.Date(2025, 6, 1, 9, 30, 0, 0, utc)},
	}
	for _, c := range cases {
		res, err := Parse(c.in, utc, now)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.in, err)
		}
		if !res.Time.Equal(c.want) {
			t.Errorf("Parse(%q) = %v, want %v", c.in, res.Time, c.want)
		}
	}
}

// T16: empty and "now" use the injected clock in the source zone.
func TestNow(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	for _, in := range []string{"", "now", "NOW"} {
		res, err := Parse(in, ny, now)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if !res.Time.Equal(now) {
			t.Errorf("Parse(%q) = %v, want %v", in, res.Time, now)
		}
		if res.Time.Location() != ny {
			t.Errorf("Parse(%q) location = %v, want America/New_York", in, res.Time.Location())
		}
	}
}

// T17: garbage input errors and lists accepted layouts.
func TestUnparseable(t *testing.T) {
	_, err := Parse("tomorrowish", time.UTC, time.Now())
	if err == nil {
		t.Fatal("expected parse error")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("error type = %T, want *ParseError", err)
	}
}

// RFC3339 with explicit offset is honoured as an absolute instant.
func TestRFC3339Absolute(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	res, err := Parse("2025-01-15T12:00:00Z", ny, time.Now())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	if !res.Time.Equal(want) {
		t.Errorf("got %v, want %v", res.Time.UTC(), want)
	}
}
