package svghtml

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitgraph/internal/calendar"
	"gitgraph/internal/model"
)

var update = flag.Bool("update", false, "update golden files")

func fixture() (model.Calendar, model.Stats) {
	start := model.Date{Year: 2026, Month: 3, Day: 1}
	end := model.Date{Year: 2026, Month: 3, Day: 21}
	counts := map[model.Date]int{
		{Year: 2026, Month: 3, Day: 1}:  1,
		{Year: 2026, Month: 3, Day: 3}:  3,
		{Year: 2026, Month: 3, Day: 7}:  4,
		{Year: 2026, Month: 3, Day: 21}: 5,
	}
	cal := calendar.Build(counts, start, end, time.Sunday)
	stats := model.Stats{Total: 13, ActiveDays: 4, Max: 5, CurrentStreak: 1, LongestStreak: 1,
		Busiest: model.Date{Year: 2026, Month: 3, Day: 21}, Days: 21}
	return cal, stats
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
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
		t.Errorf("output mismatch for %s\n--- got ---\n%s", name, got)
	}
}

func TestSVGGolden(t *testing.T) {
	cal, _ := fixture()
	checkGolden(t, "graph.svg.golden", []byte(SVG(cal)))
}

func TestHTMLGolden(t *testing.T) {
	cal, stats := fixture()
	checkGolden(t, "graph.html.golden", []byte(HTML(cal, stats, "Test Graph")))
}

func TestSVGStructure(t *testing.T) {
	cal, _ := fixture()
	out := SVG(cal)
	if !strings.HasPrefix(out, "<svg") || !strings.HasSuffix(strings.TrimSpace(out), "</svg>") {
		t.Error("SVG not well-formed wrapper")
	}
	// 3 weeks * 7 days = 21 rect cells.
	if n := strings.Count(out, "<rect"); n != 21 {
		t.Errorf("rect count = %d, want 21", n)
	}
	// Highest-intensity colour must appear (day with 5 commits == max).
	if !strings.Contains(out, levelColor[4]) {
		t.Error("expected top-level colour in SVG")
	}
}
