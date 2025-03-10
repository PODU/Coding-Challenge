// Command tzc converts a time from one timezone into others. It can also show
// a 24-hour planner grid or convert a time range.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	// Use the bundled tz database so results don't depend on the host's copy.
	_ "time/tzdata"

	"tzc/internal/convert"
	"tzc/internal/parser"
	"tzc/internal/render"
	"tzc/internal/zones"
)

const (
	exitOK          = 0
	exitUsage       = 1
	exitUnknownZone = 2
	exitBadTime     = 3
)

type config struct {
	timeStr    string
	from       string
	format     string
	rng        string
	grid       bool
	sortZones  bool
	list       bool
	jsonOut    bool
	twelveHour bool
	targets    []string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		usage(stderr)
		return exitUsage
	}

	if cfg.list {
		printList(stdout)
		return exitOK
	}

	srcRes, err := zones.Resolve(cfg.from)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUnknownZone
	}

	if len(cfg.targets) == 0 {
		fmt.Fprintln(stderr, "error: no target zones given")
		usage(stderr)
		return exitUsage
	}

	layout, err := render.ResolveLayout(cfg.format, cfg.twelveHour)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitUsage
	}
	opt := render.Options{Layout: layout, TwelveHour: cfg.twelveHour}

	// The source zone is listed first, followed by the targets.
	zoneList := []zones.Zone{srcRes.Zone()}
	var notes []string
	if srcRes.Note != "" {
		notes = append(notes, fmt.Sprintf("source %q: %s", cfg.from, srcRes.Note))
	}
	for _, tok := range cfg.targets {
		res, err := zones.Resolve(tok)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitUnknownZone
		}
		zoneList = append(zoneList, res.Zone())
		if res.Note != "" {
			notes = append(notes, fmt.Sprintf("%q: %s", tok, res.Note))
		}
	}

	switch {
	case cfg.rng != "":
		return runRange(cfg, srcRes, zoneList, notes, opt, layout, stdout, stderr, now)
	case cfg.grid:
		return runGrid(cfg, srcRes, zoneList, notes, opt, layout, stdout, stderr, now)
	default:
		return runConvert(cfg, srcRes, zoneList, notes, opt, layout, stdout, stderr, now)
	}
}

func runConvert(cfg config, src zones.Resolution, zoneList []zones.Zone, notes []string, opt render.Options, layout string, stdout, stderr io.Writer, now time.Time) int {
	res, err := parser.Parse(cfg.timeStr, src.Loc, now)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitBadTime
	}

	cs := convert.Convert(res.Time, zoneList, src.Loc)
	if cfg.sortZones {
		convert.SortByOffset(cs)
	}
	warnings := dstWarnings(res, src, notes)

	if cfg.jsonOut {
		out := render.Output{
			Source:     src.Name,
			SourceTime: res.Time.Format(time.RFC3339),
			Warnings:   warnings,
		}
		for _, c := range cs {
			out.Conversions = append(out.Conversions, render.ToConversionJSON(c, layout))
		}
		if err := render.JSON(stdout, out); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitUsage
		}
		return exitOK
	}

	header := fmt.Sprintf("Source: %s  %s", src.Name, res.Time.Format(layout))
	render.Conversions(stdout, header, cs, warnings, opt)
	return exitOK
}

func runGrid(cfg config, src zones.Resolution, zoneList []zones.Zone, notes []string, opt render.Options, layout string, stdout, stderr io.Writer, now time.Time) int {
	res, err := parser.Parse(cfg.timeStr, src.Loc, now)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitBadTime
	}

	// Sort first: the grid's hours start at midnight in whichever zone ends
	// up first.
	if cfg.sortZones {
		cs := convert.Convert(res.Time, zoneList, src.Loc)
		convert.SortByOffset(cs)
		zoneList = zoneList[:0]
		for _, c := range cs {
			zoneList = append(zoneList, zones.Zone{Name: c.Zone, Loc: c.Loc})
		}
	}
	g := convert.BuildGrid(res.Time, zoneList)

	if cfg.jsonOut {
		out := render.Output{
			Source:     src.Name,
			SourceTime: res.Time.Format(time.RFC3339),
			Warnings:   notes,
			Grid:       render.ToGridJSON(g, layout),
		}
		if err := render.JSON(stdout, out); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitUsage
		}
		return exitOK
	}
	for _, n := range notes {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	render.GridView(stdout, g, opt)
	return exitOK
}

func runRange(cfg config, src zones.Resolution, zoneList []zones.Zone, notes []string, opt render.Options, layout string, stdout, stderr io.Writer, now time.Time) int {
	startStr, endStr, ok := strings.Cut(cfg.rng, "..")
	if !ok {
		fmt.Fprintln(stderr, "error: --range must be START..END")
		return exitUsage
	}
	startRes, err := parser.Parse(strings.TrimSpace(startStr), src.Loc, now)
	if err != nil {
		fmt.Fprintln(stderr, "error: range start:", err)
		return exitBadTime
	}
	endRes, err := parser.Parse(strings.TrimSpace(endStr), src.Loc, now)
	if err != nil {
		fmt.Fprintln(stderr, "error: range end:", err)
		return exitBadTime
	}

	rows := convert.BuildRange(startRes.Time, endRes.Time, zoneList, src.Loc, cfg.sortZones)
	warnings := notes
	if w := warnForResult(startRes, src, "range start"); w != "" {
		warnings = append(warnings, w)
	}
	if w := warnForResult(endRes, src, "range end"); w != "" {
		warnings = append(warnings, w)
	}

	if cfg.jsonOut {
		out := render.Output{
			Source:     src.Name,
			SourceTime: startRes.Time.Format(time.RFC3339),
			Warnings:   warnings,
		}
		for _, r := range rows {
			out.Range = append(out.Range, render.RangeJSON{
				Zone:  r.Start.Zone,
				Start: render.ToConversionJSON(r.Start, layout),
				End:   render.ToConversionJSON(r.End, layout),
			})
		}
		if err := render.JSON(stdout, out); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitUsage
		}
		return exitOK
	}
	render.RangeView(stdout, rows, warnings, opt)
	return exitOK
}

// dstWarnings returns the DST warning for the source time, if any, followed by
// the zone notes.
func dstWarnings(res parser.Result, src zones.Resolution, notes []string) []string {
	var warnings []string
	if w := warnForResult(res, src, ""); w != "" {
		warnings = append(warnings, w)
	}
	warnings = append(warnings, notes...)
	return warnings
}

// warnForResult describes how a time that fell in a DST gap or overlap was
// interpreted, or returns "" if it didn't.
func warnForResult(res parser.Result, src zones.Resolution, label string) string {
	if !res.Gap && !res.Ambiguous {
		return ""
	}
	abbr, off := res.Time.Zone()
	prefix := ""
	if label != "" {
		prefix = label + ": "
	}
	if res.Gap {
		return fmt.Sprintf("%s%q does not exist in %s (spring-forward gap); using %s %s (%s)",
			prefix, res.Input, src.Name, res.Time.Format("15:04"), abbr, render.FormatOffset(off))
	}
	return fmt.Sprintf("%s%q is ambiguous in %s (fall-back overlap); using earlier offset %s (%s)",
		prefix, res.Input, src.Name, abbr, render.FormatOffset(off))
}

func printList(w io.Writer) {
	cities, abbrevs := zones.ListAliases()
	fmt.Fprintln(w, "City aliases:")
	for _, c := range cities {
		fmt.Fprintln(w, "  "+c)
	}
	fmt.Fprintln(w, "\nAbbreviations:")
	for _, a := range abbrevs {
		fmt.Fprintln(w, "  "+a)
	}
	fmt.Fprintln(w, "\nAny full IANA zone name (e.g. America/New_York, Asia/Kolkata) is also accepted.")
}

// parseArgs accepts both "--flag=value" and "--flag value". Anything that
// isn't a flag is a target zone.
func parseArgs(args []string) (config, error) {
	cfg := config{from: "UTC", sortZones: true}
	i := 0
	next := func(flag string) (string, error) {
		if i+1 >= len(args) {
			return "", fmt.Errorf("flag %s needs a value", flag)
		}
		i++
		return args[i], nil
	}
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			cfg.targets = append(cfg.targets, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") {
			cfg.targets = append(cfg.targets, a)
			continue
		}
		name, val, hasVal := a, "", false
		if eq := strings.IndexByte(a, '='); eq >= 0 {
			name, val, hasVal = a[:eq], a[eq+1:], true
		}
		get := func() (string, error) {
			if hasVal {
				return val, nil
			}
			return next(name)
		}
		var err error
		switch name {
		case "-t", "--time":
			cfg.timeStr, err = get()
		case "-f", "--from":
			cfg.from, err = get()
		case "--format":
			cfg.format, err = get()
		case "--range":
			cfg.rng, err = get()
		case "--grid":
			cfg.grid = true
		case "--no-sort":
			cfg.sortZones = false
		case "--sort":
			cfg.sortZones = true
		case "--list":
			cfg.list = true
		case "--json":
			cfg.jsonOut = true
		case "--12h":
			cfg.twelveHour = true
		case "-h", "--help":
			return cfg, fmt.Errorf("help requested")
		default:
			return cfg, fmt.Errorf("unknown flag %q", a)
		}
		if err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: tzc [flags] <target-zone> [<target-zone> ...]

flags:
  -t, --time STR     source time (default: now); e.g. "2025-03-09 09:00", "15:04", "now"
  -f, --from ZONE    source timezone (IANA / city / abbreviation); default UTC
      --format FMT   output layout: kitchen | 24h | 12h | iso | <Go layout>
      --12h          12-hour clock with AM/PM
      --grid         render the aligned 24-hour planner grid
      --range A..B   convert a START..END range, shown per zone
      --no-sort      keep target order (default sorts west->east by offset)
      --list         list known city aliases and abbreviations, then exit
      --json         machine-readable output

examples:
  tzc -t "2025-03-09 09:00" -f "new york" london tokyo
  tzc --grid -f "san francisco" london berlin tokyo sydney
  tzc --range "09:00..17:00" -f london new_york tokyo --json
`)
}
