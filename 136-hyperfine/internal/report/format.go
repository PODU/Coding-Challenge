package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"cchyperfine/internal/bench"
	"cchyperfine/internal/stats"
)

// labelWidth aligns the numbers on the "Time" and "Range" lines into the same
// column even though their labels differ in length.
const labelWidth = 18

// Reporter writes human-readable benchmark output.
type Reporter struct {
	Out        io.Writer
	Style      Style
	Unit       Unit
	UnitForced bool
}

// Header announces the benchmark that is about to run.
func (r *Reporter) Header(index int, command string) {
	fmt.Fprintf(r.Out, "%s\n", r.Style.bold(fmt.Sprintf("Benchmark %d: %s", index+1, command)))
}

// unitFor resolves the display unit for one result, honouring `--time-unit`.
func (r *Reporter) unitFor(mean float64) Unit {
	if r.UnitForced {
		return r.Unit
	}
	return UnitFor(mean)
}

// Result prints the statistics block for one finished benchmark.
func (r *Reporter) Result(res *bench.Result) {
	s := r.Style
	u := r.unitFor(res.Wall.Mean)
	suffix := u.Short()
	if !s.Unicode {
		suffix = u.ASCII()
	}

	cpu := fmt.Sprintf("[User: %s %s, System: %s %s]",
		u.Format(res.UserT.Mean), suffix, u.Format(res.SysT.Mean), suffix)

	pm := s.sym("±", "+-")

	if len(res.Times) == 1 {
		label := "Time (abs " + s.sym("≡", "=") + "):"
		fmt.Fprintf(r.Out, "  %s%s %s               %s\n",
			padLabel(s.greenB(label), label, labelWidth),
			s.greenB(Pad(u.Format(res.Wall.Mean), 8)), suffix, s.blue(cpu))
	} else {
		label := "Time (mean " + pm + " " + s.sym("σ", "sd") + "):"
		fmt.Fprintf(r.Out, "  %s%s %s %s %s %s    %s\n",
			padLabel(s.greenB(label), label, labelWidth),
			s.greenB(Pad(u.Format(res.Wall.Mean), 8)), suffix,
			pm,
			s.green(Pad(u.Format(res.Wall.StdDev), 6)), suffix,
			s.blue(cpu))

		rangeLabel := "Range (min " + s.sym("…", "...") + " max):"
		fmt.Fprintf(r.Out, "  %s%s %s %s %s %s    %d runs\n",
			padLabel(rangeLabel, rangeLabel, labelWidth),
			s.cyan(Pad(u.Format(res.Wall.Min), 8)), suffix,
			s.sym("…", "..."),
			s.magenta(u.Format(res.Wall.Max)), suffix,
			len(res.Times))
	}

	for _, a := range res.Alerts {
		fmt.Fprintf(r.Out, "\n  %s %s\n", s.yellow(s.sym("⚠", "!")+" Warning:"), a.Message)
	}
	fmt.Fprintln(r.Out)
}

// padLabel right-pads a possibly-coloured label to width. The padding is
// measured on the plain text, because ANSI escapes and multi-byte glyphs both
// lie about a string's printed width.
func padLabel(rendered, plain string, width int) string {
	if n := utf8.RuneCountInString(plain); n < width {
		return rendered + strings.Repeat(" ", width-n)
	}
	return rendered
}

// Comparison is one line of the relative-speed summary.
type Comparison struct {
	Result *bench.Result
	Ratio  float64
	Error  float64
}

// Compare ranks results against a baseline: the explicitly named `--reference`
// if there is one, otherwise the fastest command.
func Compare(results []*bench.Result, reference string) (baseline *bench.Result, others []Comparison) {
	if len(results) == 0 {
		return nil, nil
	}
	sorted := append([]*bench.Result(nil), results...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Wall.Mean < sorted[j].Wall.Mean })

	baseline = sorted[0]
	if reference != "" {
		for _, res := range results {
			if res.Command == reference || res.Name == reference {
				baseline = res
				break
			}
		}
	}

	for _, res := range sorted {
		if res == baseline {
			continue
		}
		ratio := 0.0
		if baseline.Wall.Mean > 0 {
			ratio = res.Wall.Mean / baseline.Wall.Mean
		}
		err := stats.RatioError(ratio, res.Wall.Mean, res.Wall.StdDev, baseline.Wall.Mean, baseline.Wall.StdDev)
		others = append(others, Comparison{Result: res, Ratio: ratio, Error: err})
	}
	return baseline, others
}

// Summary prints the relative-speed block shown after two or more benchmarks.
func (r *Reporter) Summary(results []*bench.Result, reference string) {
	if len(results) < 2 {
		return
	}
	baseline, others := Compare(results, reference)
	s := r.Style
	fmt.Fprintf(r.Out, "%s\n", s.bold("Summary"))
	fmt.Fprintf(r.Out, "  %s ran\n", s.cyan(baseline.Name))
	for _, c := range others {
		fmt.Fprintf(r.Out, "    %s %s %s times faster than %s\n",
			s.greenB(fmt.Sprintf("%.2f", c.Ratio)),
			s.sym("±", "+-"),
			s.green(fmt.Sprintf("%.2f", c.Error)),
			s.magenta(c.Result.Name))
	}
}

// SortResults applies `--sort` to the result list used for the summary and the
// exports. "auto" preserves the order the commands were given in.
func SortResults(results []*bench.Result, method string) []*bench.Result {
	out := append([]*bench.Result(nil), results...)
	switch method {
	case "command":
		sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	case "mean-time":
		sort.SliceStable(out, func(i, j int) bool { return out[i].Wall.Mean < out[j].Wall.Mean })
	}
	return out
}
