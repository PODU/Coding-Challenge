package terminal

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitgraph/internal/calendar"
	"gitgraph/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

// fixture builds a deterministic calendar + stats with no git involved.
func fixture() (model.Calendar, model.Stats) {
	start := model.Date{Year: 2026, Month: 3, Day: 1} // Sunday
	end := model.Date{Year: 2026, Month: 3, Day: 21}
	counts := map[model.Date]int{
		{Year: 2026, Month: 3, Day: 1}:  1,
		{Year: 2026, Month: 3, Day: 2}:  2,
		{Year: 2026, Month: 3, Day: 3}:  3,
		{Year: 2026, Month: 3, Day: 7}:  4,
		{Year: 2026, Month: 3, Day: 10}: 1,
		{Year: 2026, Month: 3, Day: 18}: 2,
		{Year: 2026, Month: 3, Day: 21}: 5,
	}
	cal := calendar.Build(counts, start, end, time.Sunday)
	stats := model.Stats{
		Total: 18, ActiveDays: 7, Max: 5,
		CurrentStreak: 1, LongestStreak: 3,
		Busiest: model.Date{Year: 2026, Month: 3, Day: 21}, Days: 21,
	}
	return cal, stats
}

func goldenPath(name string) string { return filepath.Join("testdata", name) }

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run -update to create): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestRenderNoColorGolden(t *testing.T) {
	cal, stats := fixture()
	var buf bytes.Buffer
	if err := Render(&buf, cal, stats, Options{Color: false, Title: "Test Graph"}); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "terminal_nocolor.golden", buf.Bytes())
}

func TestRenderColorGolden(t *testing.T) {
	cal, stats := fixture()
	var buf bytes.Buffer
	if err := Render(&buf, cal, stats, Options{Color: true, Title: "Test Graph"}); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "terminal_color.golden", buf.Bytes())
}

func TestRenderStatsOnly(t *testing.T) {
	_, stats := fixture()
	var buf bytes.Buffer
	if err := Render(&buf, model.Calendar{}, stats, Options{StatsOnly: true}); err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "terminal_statsonly.golden", buf.Bytes())
}
