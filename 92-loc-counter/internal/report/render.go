package report

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// RenderText prints the table, with one row per file instead of per language
// if byFile is set.
func RenderText(w io.Writer, rep Report, byFile bool) {
	firstHdr := "Language"
	if byFile {
		firstHdr = "File"
	}

	type cells [6]string
	num := func(n int) string { return strconv.Itoa(n) }
	mk := func(name string, files, lines, blank, comment, code int) cells {
		return cells{name, num(files), num(lines), num(blank), num(comment), num(code)}
	}

	header := cells{firstHdr, "Files", "Lines", "Blanks", "Comments", "Code"}
	var body []cells
	if byFile {
		for _, f := range rep.Files {
			body = append(body, mk(f.Path, 1, f.Lines, f.Blank, f.Comment, f.Code))
		}
	} else {
		for _, l := range rep.Languages {
			body = append(body, mk(l.Language, l.Files, l.Lines, l.Blank, l.Comment, l.Code))
		}
	}
	t := rep.Total
	total := mk("Total", t.Files, t.Lines, t.Blank, t.Comment, t.Code)

	var width [6]int
	consider := func(c cells) {
		for i, s := range c {
			if len(s) > width[i] {
				width[i] = len(s)
			}
		}
	}
	consider(header)
	consider(total)
	for _, r := range body {
		consider(r)
	}

	const gap = 2
	lineWidth := 1 + 5*gap // leading space and the gaps between six columns
	for _, wd := range width {
		lineWidth += wd
	}
	rule := strings.Repeat("-", lineWidth)

	printRow := func(c cells) {
		var b strings.Builder
		b.WriteByte(' ')
		b.WriteString(c[0]) // the name is left-aligned, numbers right-aligned
		b.WriteString(strings.Repeat(" ", width[0]-len(c[0])))
		for i := 1; i < 6; i++ {
			b.WriteString(strings.Repeat(" ", gap))
			b.WriteString(strings.Repeat(" ", width[i]-len(c[i])))
			b.WriteString(c[i])
		}
		fmt.Fprintln(w, b.String())
	}

	fmt.Fprintln(w, rule)
	printRow(header)
	fmt.Fprintln(w, rule)
	for _, r := range body {
		printRow(r)
	}
	fmt.Fprintln(w, rule)
	printRow(total)
	fmt.Fprintln(w, rule)
}

type jsonReport struct {
	Languages []LangTotal `json:"languages"`
	Total     LangTotal   `json:"total"`
	Files     []jsonFile  `json:"files,omitempty"`
}

type jsonFile struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
	Blank    int    `json:"blank"`
	Comment  int    `json:"comment"`
	Code     int    `json:"code"`
}

// RenderJSON writes the report as JSON, with per-file counts if includeFiles
// is set.
func RenderJSON(w io.Writer, rep Report, includeFiles bool) error {
	out := jsonReport{Languages: rep.Languages, Total: rep.Total}
	if includeFiles {
		for _, f := range rep.Files {
			out.Files = append(out.Files, jsonFile{
				Path: f.Path, Language: f.Lang, Lines: f.Lines,
				Blank: f.Blank, Comment: f.Comment, Code: f.Code,
			})
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// Cocomo is a basic COCOMO estimate for an organic project.
type Cocomo struct {
	KLOC         float64
	EffortMonths float64
	ScheduleMos  float64
	People       float64
	Cost         float64
}

// EstimateCocomo estimates effort and cost from the lines of code. avgWage is
// an annual salary.
func EstimateCocomo(codeLines int, avgWage float64) Cocomo {
	kloc := float64(codeLines) / 1000.0
	effort := 2.4 * math.Pow(kloc, 1.05)
	schedule := 2.5 * math.Pow(effort, 0.38)
	people := 0.0
	if schedule > 0 {
		people = effort / schedule
	}
	cost := effort * (avgWage / 12.0)
	return Cocomo{KLOC: kloc, EffortMonths: effort, ScheduleMos: schedule, People: people, Cost: cost}
}

// RenderCocomo prints the estimate.
func RenderCocomo(w io.Writer, c Cocomo, avgWage float64) {
	fmt.Fprintf(w, "Estimated Cost to Develop (organic) $%.0f\n", c.Cost)
	fmt.Fprintf(w, "Estimated Schedule Effort (organic) %.2f months\n", c.ScheduleMos)
	fmt.Fprintf(w, "Estimated People Required (organic) %.2f\n", c.People)
	fmt.Fprintf(w, "  (effort %.2f person-months, KLOC %.2f, avg wage $%.0f/yr)\n",
		c.EffortMonths, c.KLOC, avgWage)
}
