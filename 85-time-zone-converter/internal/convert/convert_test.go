package convert

import (
	"testing"
	"time"

	"tzc/internal/parser"
	"tzc/internal/zones"
)

func zone(t *testing.T, name string) zones.Zone {
	t.Helper()
	res, err := zones.Resolve(name)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", name, err)
	}
	return res.Zone()
}

// parseIn parses a wall-clock string in the named zone.
func parseIn(t *testing.T, s, zoneName string) (time.Time, *time.Location) {
	t.Helper()
	z := zone(t, zoneName)
	res, err := parser.Parse(s, z.Loc, time.Now())
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return res.Time, z.Loc
}

// convertTo converts an instant into a single named target zone.
func convertTo(t *testing.T, instant time.Time, src *time.Location, target string) Conversion {
	t.Helper()
	cs := Convert(instant, []zones.Zone{zone(t, target)}, src)
	return cs[0]
}

// T1-T5: golden conversions.
func TestGoldenConversions(t *testing.T) {
	cases := []struct {
		id           string
		in, from, to string
		wantWall     string
		wantDayShift int
	}{
		{"T1", "2025-01-15 12:00", "America/New_York", "Europe/London", "17:00", 0},
		{"T2", "2025-07-15 12:00", "America/New_York", "Europe/London", "17:00", 0},
		{"T3", "2025-01-15 23:00", "America/Los_Angeles", "Asia/Tokyo", "16:00", 1},
		{"T4", "2025-06-01 09:00", "UTC", "Asia/Kolkata", "14:30", 0},
		{"T5", "2025-06-01 09:00", "UTC", "Asia/Kathmandu", "14:45", 0},
	}
	for _, c := range cases {
		instant, src := parseIn(t, c.in, c.from)
		got := convertTo(t, instant, src, c.to)
		if w := got.Time.Format("15:04"); w != c.wantWall {
			t.Errorf("%s: wall = %q, want %q", c.id, w, c.wantWall)
		}
		if got.DayShift != c.wantDayShift {
			t.Errorf("%s: dayShift = %d, want %d", c.id, got.DayShift, c.wantDayShift)
		}
	}
}

// T6: EU has not yet sprung on 2025-03-12, so NY->London delta is 4h not 5h.
func TestEUvsUSSpringGap(t *testing.T) {
	instant, src := parseIn(t, "2025-03-12 09:00", "America/New_York")
	got := convertTo(t, instant, src, "Europe/London")
	if w := got.Time.Format("15:04"); w != "13:00" {
		t.Errorf("London wall = %q, want 13:00 (delta 4h)", w)
	}
	if got.DeltaSec != 4*3600 {
		t.Errorf("delta = %d, want 14400 (4h)", got.DeltaSec)
	}
}

// T9: Southern-hemisphere inverse DST for Australia/Sydney.
func TestSydneyInverseDST(t *testing.T) {
	sydney := zone(t, "Australia/Sydney")
	jul := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	jan := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, offJul := jul.In(sydney.Loc).Zone()
	_, offJan := jan.In(sydney.Loc).Zone()
	if offJul != 10*3600 {
		t.Errorf("Jul offset = %d, want +36000 (+10:00)", offJul)
	}
	if offJan != 11*3600 {
		t.Errorf("Jan offset = %d, want +39600 (+11:00)", offJan)
	}
}

// T10: IsDST is true for London in July, false in January.
func TestIsDSTFlag(t *testing.T) {
	jul, src := parseIn(t, "2025-07-15 12:00", "Europe/London")
	jan, _ := parseIn(t, "2025-01-15 12:00", "Europe/London")
	if !convertTo(t, jul, src, "Europe/London").IsDST {
		t.Errorf("London 2025-07-15 should be DST")
	}
	if convertTo(t, jan, src, "Europe/London").IsDST {
		t.Errorf("London 2025-01-15 should not be DST")
	}
}

// T18: sort orders zones west->east relative to the source.
func TestSortByOffset(t *testing.T) {
	instant, src := parseIn(t, "2025-06-01 12:00", "Europe/London")
	zs := []zones.Zone{
		zone(t, "Asia/Tokyo"),
		zone(t, "Europe/London"),
		zone(t, "America/New_York"),
	}
	cs := Convert(instant, zs, src)
	SortByOffset(cs)
	wantOrder := []string{"America/New_York", "Europe/London", "Asia/Tokyo"}
	for i, w := range wantOrder {
		if cs[i].Zone != w {
			t.Errorf("position %d = %q, want %q", i, cs[i].Zone, w)
		}
	}
}

// T19: grid has 24 cells per zone and column 0 is 00:00 in the first zone.
func TestGridLayout(t *testing.T) {
	instant, _ := parseIn(t, "2025-06-01 12:00", "America/New_York")
	zs := []zones.Zone{zone(t, "America/New_York"), zone(t, "Europe/London")}
	g := BuildGrid(instant, zs)
	if len(g.Zones) != 2 {
		t.Fatalf("rows = %d, want 2", len(g.Zones))
	}
	for _, row := range g.Zones {
		if len(row.Cells) != 24 {
			t.Errorf("%s: %d cells, want 24", row.Name, len(row.Cells))
		}
	}
	if h := g.Zones[0].Cells[0].Time.Hour(); h != 0 {
		t.Errorf("first cell hour = %d, want 0", h)
	}
	// London is +5h ahead of NY in June: NY 00:00 -> London 05:00.
	if h := g.Zones[1].Cells[0].Time.Hour(); h != 5 {
		t.Errorf("London first cell hour = %d, want 5", h)
	}
}

// T20: range preserves the delta between start and end per zone.
func TestRangeView(t *testing.T) {
	src := zone(t, "Europe/London")
	start, _ := parseIn(t, "2025-06-01 09:00", "Europe/London")
	end, _ := parseIn(t, "2025-06-01 17:00", "Europe/London")
	zs := []zones.Zone{src, zone(t, "America/New_York"), zone(t, "Asia/Tokyo")}
	rows := BuildRange(start, end, zs, src.Loc, true)
	for _, r := range rows {
		dur := r.End.Time.Sub(r.Start.Time)
		if dur != 8*time.Hour {
			t.Errorf("%s: range = %v, want 8h", r.Start.Zone, dur)
		}
	}
	// New York at 09:00 London (BST +1) is 04:00 EDT (-4): delta 5h.
	for _, r := range rows {
		if r.Start.Zone == "America/New_York" {
			if w := r.Start.Time.Format("15:04"); w != "04:00" {
				t.Errorf("NY start = %q, want 04:00", w)
			}
		}
	}
}

// T22: round-trip A->B then B->A yields the original instant.
func TestRoundTrip(t *testing.T) {
	instant, _ := parseIn(t, "2025-03-30 06:30", "UTC")
	for _, name := range []string{"America/New_York", "Asia/Kolkata", "Australia/Sydney", "Pacific/Auckland"} {
		z := zone(t, name)
		there := instant.In(z.Loc)
		back := there.In(time.UTC)
		if !back.Equal(instant) {
			t.Errorf("%s round-trip: got %v, want %v", name, back, instant)
		}
	}
}

// T23: host-independence — conversions depend only on explicit zones, never on
// the host TZ. Setting TZ must not change the computed instant.
func TestHostIndependence(t *testing.T) {
	t.Setenv("TZ", "Asia/Kolkata")
	instant, src := parseIn(t, "2025-01-15 12:00", "America/New_York")
	got := convertTo(t, instant, src, "Europe/London")
	wantUTC := time.Date(2025, 1, 15, 17, 0, 0, 0, time.UTC)
	if !got.Time.UTC().Equal(wantUTC) {
		t.Errorf("got %v, want %v regardless of host TZ", got.Time.UTC(), wantUTC)
	}
}
