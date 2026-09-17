// Package export writes benchmark results to machine- and document-friendly
// files.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"cchyperfine/internal/bench"
	"cchyperfine/internal/report"
)

// Writer collects the destinations requested on the command line.
type Writer struct {
	JSON     string
	CSV      string
	Markdown string
	Asciidoc string
	Orgmode  string
}

// WriteAll emits every requested export. It keeps going after a failure so
// that one unwritable path does not silently drop the others, returning the
// first error encountered.
func (w Writer) WriteAll(results []*bench.Result) error {
	var first error
	record := func(err error) {
		if err != nil && first == nil {
			first = err
		}
	}
	if w.JSON != "" {
		record(writeFile(w.JSON, func(f *os.File) error { return writeJSON(f, results) }))
	}
	if w.CSV != "" {
		record(writeFile(w.CSV, func(f *os.File) error { return writeCSV(f, results) }))
	}
	if w.Markdown != "" {
		record(writeFile(w.Markdown, func(f *os.File) error { return writeMarkdown(f, results) }))
	}
	if w.Asciidoc != "" {
		record(writeFile(w.Asciidoc, func(f *os.File) error { return writeAsciidoc(f, results) }))
	}
	if w.Orgmode != "" {
		record(writeFile(w.Orgmode, func(f *os.File) error { return writeOrgmode(f, results) }))
	}
	return first
}

func writeFile(path string, fn func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	defer f.Close()
	if err := fn(f); err != nil {
		return fmt.Errorf("export %s: %w", path, err)
	}
	return nil
}

// jsonResult mirrors hyperfine's JSON schema so existing tooling can read it.
type jsonResult struct {
	Command    string            `json:"command"`
	Mean       float64           `json:"mean"`
	StdDev     float64           `json:"stddev"`
	Median     float64           `json:"median"`
	User       float64           `json:"user"`
	System     float64           `json:"system"`
	Min        float64           `json:"min"`
	Max        float64           `json:"max"`
	Times      []float64         `json:"times"`
	ExitCodes  []int             `json:"exit_codes"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

func writeJSON(f *os.File, results []*bench.Result) error {
	out := struct {
		Results []jsonResult `json:"results"`
	}{Results: make([]jsonResult, 0, len(results))}

	for _, r := range results {
		out.Results = append(out.Results, jsonResult{
			Command:    r.Name,
			Mean:       r.Wall.Mean,
			StdDev:     r.Wall.StdDev,
			Median:     r.Wall.Median,
			User:       r.UserT.Mean,
			System:     r.SysT.Mean,
			Min:        r.Wall.Min,
			Max:        r.Wall.Max,
			Times:      r.Times,
			ExitCodes:  r.ExitCodes,
			Parameters: r.Parameters,
		})
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	// Commands routinely contain <, > and & as redirections; escaping them
	// would make the export unreadable for no benefit outside a browser.
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

func writeCSV(f *os.File, results []*bench.Result) error {
	params := parameterNames(results)
	w := csv.NewWriter(f)

	header := []string{"command", "mean", "stddev", "median", "user", "system", "min", "max"}
	for _, p := range params {
		header = append(header, "parameter_"+p)
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range results {
		row := []string{
			r.Name,
			num(r.Wall.Mean), num(r.Wall.StdDev), num(r.Wall.Median),
			num(r.UserT.Mean), num(r.SysT.Mean),
			num(r.Wall.Min), num(r.Wall.Max),
		}
		for _, p := range params {
			row = append(row, r.Parameters[p])
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// parameterNames collects every parameter used across results, so the CSV has
// a stable column set even if some rows lack a value.
func parameterNames(results []*bench.Result) []string {
	seen := map[string]bool{}
	var names []string
	for _, r := range results {
		for k := range r.Parameters {
			if !seen[k] {
				seen[k] = true
				names = append(names, k)
			}
		}
	}
	sort.Strings(names)
	return names
}

// tableRow is the shared shape of the markdown/asciidoc/orgmode tables.
type tableRow struct {
	Command  string
	Mean     string
	StdDev   string
	Min      string
	Max      string
	Relative string
}

// tableRows renders the results in a single display unit -- chosen from the
// fastest mean so the table's header says "[ms]" once and every cell agrees.
func tableRows(results []*bench.Result) (rows []tableRow, unit report.Unit) {
	unit = report.Second
	if len(results) > 0 {
		fastest := results[0].Wall.Mean
		for _, r := range results {
			if r.Wall.Mean < fastest {
				fastest = r.Wall.Mean
			}
		}
		unit = report.UnitFor(fastest)
	}

	baseline, comparisons := report.Compare(results, "")
	rel := map[*bench.Result]string{}
	if baseline != nil {
		rel[baseline] = "1.00"
	}
	for _, c := range comparisons {
		rel[c.Result] = fmt.Sprintf("%.2f ± %.2f", c.Ratio, c.Error)
	}

	for _, r := range results {
		rows = append(rows, tableRow{
			Command:  r.Name,
			Mean:     unit.Format(r.Wall.Mean),
			StdDev:   unit.Format(r.Wall.StdDev),
			Min:      unit.Format(r.Wall.Min),
			Max:      unit.Format(r.Wall.Max),
			Relative: rel[r],
		})
	}
	return rows, unit
}

func writeMarkdown(f *os.File, results []*bench.Result) error {
	rows, unit := tableRows(results)
	u := unit.Short()
	var b strings.Builder
	fmt.Fprintf(&b, "| Command | Mean [%s] | Min [%s] | Max [%s] | Relative |\n", u, u, u)
	b.WriteString("|:---|---:|---:|---:|---:|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| `%s` | %s ± %s | %s | %s | %s |\n",
			escapeBackticks(r.Command), r.Mean, r.StdDev, r.Min, r.Max, r.Relative)
	}
	_, err := f.WriteString(b.String())
	return err
}

func writeAsciidoc(f *os.File, results []*bench.Result) error {
	rows, unit := tableRows(results)
	u := unit.Short()
	var b strings.Builder
	b.WriteString(`[cols="<,>,>,>,>"]` + "\n|===\n")
	fmt.Fprintf(&b, "| Command\n| Mean [%s]\n| Min [%s]\n| Max [%s]\n| Relative\n\n", u, u, u)
	for _, r := range rows {
		fmt.Fprintf(&b, "| `%s`\n| %s ± %s\n| %s\n| %s\n| %s\n\n",
			escapeBackticks(r.Command), r.Mean, r.StdDev, r.Min, r.Max, r.Relative)
	}
	b.WriteString("|===\n")
	_, err := f.WriteString(b.String())
	return err
}

func writeOrgmode(f *os.File, results []*bench.Result) error {
	rows, unit := tableRows(results)
	u := unit.Short()
	var b strings.Builder
	fmt.Fprintf(&b, "| Command | Mean [%s] | Min [%s] | Max [%s] | Relative |\n", u, u, u)
	b.WriteString("|--+--+--+--+--|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| =%s= | %s ± %s | %s | %s | %s |\n",
			r.Command, r.Mean, r.StdDev, r.Min, r.Max, r.Relative)
	}
	_, err := f.WriteString(b.String())
	return err
}

// escapeBackticks keeps a command containing a backtick from breaking out of
// the inline-code span it is wrapped in.
func escapeBackticks(s string) string { return strings.ReplaceAll(s, "`", "'") }
