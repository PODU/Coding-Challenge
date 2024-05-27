package calendar

import (
	"testing"
	"time"

	"gitgraph/internal/model"
)

func d(y int, m time.Month, day int) model.Date { return model.Date{Year: y, Month: m, Day: day} }

func TestLevelBucketing(t *testing.T) {
	cases := []struct {
		count, max, want int
	}{
		{0, 10, 0},
		{1, 10, 1},
		{3, 10, 2},
		{5, 10, 2},
		{6, 10, 3},
		{8, 10, 4},
		{10, 10, 4},
		{1, 1, 4},   // single max-equal commit => top bucket
		{5, 0, 0},   // no max => level 0
		{0, 0, 0},   // empty
		{99, 10, 4}, // clamp
	}
	for _, c := range cases {
		if got := Level(c.count, c.max); got != c.want {
			t.Errorf("Level(%d,%d) = %d, want %d", c.count, c.max, got, c.want)
		}
	}
}

func TestBuildWeekColumnsSundayStart(t *testing.T) {
	// 2026-03-01 is a Sunday. A 14-day window Mar 1..Mar 14 = exactly 2 weeks.
	start, end := d(2026, 3, 1), d(2026, 3, 14)
	cal := Build(map[model.Date]int{}, start, end, time.Sunday)
	if len(cal.Weeks) != 2 {
		t.Fatalf("weeks = %d, want 2", len(cal.Weeks))
	}
	if !cal.Weeks[0][0].Date.Equal(d(2026, 3, 1)) {
		t.Errorf("first cell = %s, want 2026-03-01", cal.Weeks[0][0].Date)
	}
	if !cal.Weeks[1][6].Date.Equal(d(2026, 3, 14)) {
		t.Errorf("last cell = %s, want 2026-03-14", cal.Weeks[1][6].Date)
	}
}

func TestBuildLeadingPaddingAlignsFirstColumn(t *testing.T) {
	// 2026-03-04 is a Wednesday. With Sunday start, the first column must begin
	// on Sunday 2026-03-01 with Mon..Wed being padding (out of range).
	start, end := d(2026, 3, 4), d(2026, 3, 10)
	cal := Build(map[model.Date]int{}, start, end, time.Sunday)
	first := cal.Weeks[0]
	if first[0].Date.Weekday() != time.Sunday {
		t.Fatalf("column does not start on Sunday: %s", first[0].Date)
	}
	if first[3].InRange == false {
		t.Error("Wednesday (start) should be in range")
	}
	if first[0].InRange || first[1].InRange || first[2].InRange {
		t.Error("Sun..Tue before start should be padding (out of range)")
	}
}

func TestBuildMondayFirstDay(t *testing.T) {
	// 2026-03-02 is a Monday.
	start, end := d(2026, 3, 2), d(2026, 3, 8)
	cal := Build(map[model.Date]int{}, start, end, time.Monday)
	if cal.Weeks[0][0].Date.Weekday() != time.Monday {
		t.Errorf("first cell weekday = %s, want Monday", cal.Weeks[0][0].Date.Weekday())
	}
}

func TestBuildAssignsCountsAndLevels(t *testing.T) {
	start, end := d(2026, 3, 1), d(2026, 3, 7)
	counts := map[model.Date]int{
		d(2026, 3, 1): 1,
		d(2026, 3, 4): 4, // max
	}
	cal := Build(counts, start, end, time.Sunday)
	w := cal.Weeks[0]
	if w[0].Count != 1 || w[0].Level != 1 {
		t.Errorf("Sun cell = count %d level %d, want 1/1", w[0].Count, w[0].Level)
	}
	if w[3].Count != 4 || w[3].Level != 4 {
		t.Errorf("Wed cell = count %d level %d, want 4/4", w[3].Count, w[3].Level)
	}
	if w[6].Count != 0 || w[6].Level != 0 {
		t.Errorf("Sat cell = count %d level %d, want 0/0", w[6].Count, w[6].Level)
	}
}

func TestBuildEmptyRange(t *testing.T) {
	cal := Build(map[model.Date]int{}, d(2026, 3, 10), d(2026, 3, 1), time.Sunday)
	if len(cal.Weeks) != 0 {
		t.Errorf("reversed range should yield no weeks, got %d", len(cal.Weeks))
	}
}
