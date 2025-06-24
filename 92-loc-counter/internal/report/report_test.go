package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"loc/internal/count"
)

func sampleFiles() []count.FileCount {
	return []count.FileCount{
		{Path: "a.go", Lang: "Go", Lines: 100, Blank: 10, Comment: 20, Code: 70},
		{Path: "b.go", Lang: "Go", Lines: 50, Blank: 5, Comment: 5, Code: 40},
		{Path: "c.py", Lang: "Python", Lines: 30, Blank: 3, Comment: 7, Code: 20},
		{Path: "d.txt", Lang: "Text", Lines: 10, Blank: 2, Comment: 0, Code: 8},
	}
}

func TestAggregateTotals(t *testing.T) {
	rep := Aggregate(sampleFiles(), "code")

	if len(rep.Languages) != 3 {
		t.Fatalf("got %d languages; want 3", len(rep.Languages))
	}
	// Grand total.
	tot := rep.Total
	if tot.Files != 4 || tot.Lines != 190 || tot.Blank != 20 || tot.Comment != 32 || tot.Code != 138 {
		t.Errorf("total = %+v; want Files=4 Lines=190 Blank=20 Comment=32 Code=138", tot)
	}
	// Per-language Go sum.
	var goRow LangTotal
	for _, l := range rep.Languages {
		if l.Language == "Go" {
			goRow = l
		}
	}
	if goRow.Files != 2 || goRow.Code != 110 || goRow.Lines != 150 {
		t.Errorf("Go row = %+v; want Files=2 Code=110 Lines=150", goRow)
	}
	// Σ files across languages equals total files.
	sum := 0
	for _, l := range rep.Languages {
		sum += l.Files
	}
	if sum != tot.Files {
		t.Errorf("sum of language files %d != total %d", sum, tot.Files)
	}
}

func TestAggregateSortCodeDesc(t *testing.T) {
	rep := Aggregate(sampleFiles(), "code")
	want := []string{"Go", "Python", "Text"} // 110, 20, 8
	for i, l := range rep.Languages {
		if l.Language != want[i] {
			t.Errorf("position %d = %s; want %s", i, l.Language, want[i])
		}
	}
}

func TestAggregateSortName(t *testing.T) {
	rep := Aggregate(sampleFiles(), "name")
	want := []string{"Go", "Python", "Text"}
	for i, l := range rep.Languages {
		if l.Language != want[i] {
			t.Errorf("position %d = %s; want %s", i, l.Language, want[i])
		}
	}
}

func TestRenderJSONRoundTrip(t *testing.T) {
	rep := Aggregate(sampleFiles(), "code")
	var buf bytes.Buffer
	if err := RenderJSON(&buf, rep, true); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var got jsonReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON does not parse: %v", err)
	}
	if got.Total.Code != 138 {
		t.Errorf("round-trip total code = %d; want 138", got.Total.Code)
	}
	if len(got.Files) != 4 {
		t.Errorf("round-trip files = %d; want 4", len(got.Files))
	}
}

func TestRenderJSONNoFiles(t *testing.T) {
	rep := Aggregate(sampleFiles(), "code")
	var buf bytes.Buffer
	if err := RenderJSON(&buf, rep, false); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	// The top-level "files" array carries per-file "path" entries; assert none
	// are present (the per-language rows also have a "files" count field, so we
	// key on "path" which is unique to file entries).
	if strings.Contains(buf.String(), "\"path\"") {
		t.Errorf("files array should be omitted when includeFiles=false:\n%s", buf.String())
	}
}

func TestRenderTextTotals(t *testing.T) {
	rep := Aggregate(sampleFiles(), "code")
	var buf bytes.Buffer
	RenderText(&buf, rep, false)
	out := buf.String()
	if !strings.Contains(out, "Language") || !strings.Contains(out, "Total") {
		t.Errorf("text table missing header/total:\n%s", out)
	}
	if !strings.Contains(out, "138") {
		t.Errorf("text table missing total code 138:\n%s", out)
	}
}

func TestCocomo(t *testing.T) {
	c := EstimateCocomo(10000, 56286) // 10 KLOC
	// Effort = 2.4 * 10^1.05 ~= 26.9 person-months
	if c.EffortMonths < 26 || c.EffortMonths > 28 {
		t.Errorf("effort = %.2f; want ~26.9", c.EffortMonths)
	}
	if c.People <= 0 || c.ScheduleMos <= 0 {
		t.Errorf("schedule/people must be positive: %+v", c)
	}
}
