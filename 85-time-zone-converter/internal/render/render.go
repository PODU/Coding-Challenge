// Package render formats conversion results as human-readable text, an aligned
// planner grid, or machine-readable JSON.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"tzc/internal/convert"
)

// Options control formatting.
type Options struct {
	Layout     string // Go time layout for wall-clock times
	TwelveHour bool   // 12-hour clock with AM/PM
}

// ResolveLayout maps a named or explicit layout to a Go time layout.
func ResolveLayout(name string, twelveHour bool) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		if twelveHour {
			return "2006-01-02 03:04 PM", nil
		}
		return "2006-01-02 15:04", nil
	case "kitchen":
		return time.Kitchen, nil
	case "24h":
		return "2006-01-02 15:04", nil
	case "12h":
		return "2006-01-02 03:04 PM", nil
	case "iso", "rfc3339":
		return time.RFC3339, nil
	default:
		// Treat as an explicit Go layout.
		return name, nil
	}
}

// FormatOffset renders an offset in seconds as +HH:MM / -HH:MM.
func FormatOffset(sec int) string {
	sign := "+"
	if sec < 0 {
		sign = "-"
		sec = -sec
	}
	return fmt.Sprintf("%s%02d:%02d", sign, sec/3600, (sec%3600)/60)
}

func dayMarker(shift int) string {
	switch {
	case shift > 0:
		return fmt.Sprintf(" (+%dd)", shift)
	case shift < 0:
		return fmt.Sprintf(" (%dd)", shift)
	default:
		return ""
	}
}

func dstMarker(isDST bool) string {
	if isDST {
		return "DST"
	}
	return "-"
}

// Conversions prints the per-zone conversion table.
func Conversions(w io.Writer, header string, cs []convert.Conversion, warnings []string, opt Options) {
	for _, warn := range warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}
	if header != "" {
		fmt.Fprintln(w, header)
	}
	rows := [][]string{{"ZONE", "LOCAL TIME", "OFFSET", "DST", "Δ vs src"}}
	for _, c := range cs {
		name := c.Zone
		if c.IsSource {
			name = "* " + name
		}
		rows = append(rows, []string{
			name,
			c.Time.Format(opt.Layout) + dayMarker(c.DayShift),
			FormatOffset(c.OffsetSec),
			dstMarker(c.IsDST),
			FormatOffset(c.DeltaSec),
		})
	}
	writeTable(w, rows)
}

// GridView prints the aligned 24-hour planner grid.
func GridView(w io.Writer, g convert.Grid, opt Options) {
	if len(g.Zones) == 0 {
		return
	}
	fmt.Fprintf(w, "Planner grid — columns are hours 00:00..23:00 in %s\n", g.Zones[0].Name)

	// Header row: hour-of-first-zone labels.
	hours := make([]string, 0, 24)
	for h := 0; h < 24; h++ {
		hours = append(hours, fmt.Sprintf("%02d", h))
	}

	nameW := 0
	for _, z := range g.Zones {
		label := z.Name
		if z.HasTransition {
			label += " *"
		}
		if len(label) > nameW {
			nameW = len(label)
		}
	}

	fmt.Fprintf(w, "%-*s  %s\n", nameW, "", strings.Join(hours, " "))
	for _, z := range g.Zones {
		cells := make([]string, 0, 24)
		for _, c := range z.Cells {
			cells = append(cells, fmt.Sprintf("%02d", c.Time.Hour()))
		}
		label := z.Name
		if z.HasTransition {
			label += " *"
		}
		fmt.Fprintf(w, "%-*s  %s\n", nameW, label, strings.Join(cells, " "))
	}
	hasTransition := false
	for _, z := range g.Zones {
		if z.HasTransition {
			hasTransition = true
		}
	}
	if hasTransition {
		fmt.Fprintln(w, "(* zone has a DST transition within this 24-hour window)")
	}
}

// RangeView prints start/end per zone.
func RangeView(w io.Writer, rows []convert.RangeRow, warnings []string, opt Options) {
	for _, warn := range warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}
	table := [][]string{{"ZONE", "START", "END", "OFFSET", "DST"}}
	for _, r := range rows {
		name := r.Start.Zone
		if r.Start.IsSource {
			name = "* " + name
		}
		table = append(table, []string{
			name,
			r.Start.Time.Format(opt.Layout) + dayMarker(r.Start.DayShift),
			r.End.Time.Format(opt.Layout) + dayMarker(r.End.DayShift),
			FormatOffset(r.Start.OffsetSec),
			dstMarker(r.Start.IsDST),
		})
	}
	writeTable(w, table)
}

// writeTable prints a left-aligned, space-padded table.
func writeTable(w io.Writer, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	widths := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			if i < len(widths) && len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	for _, r := range rows {
		var b strings.Builder
		for i, c := range r {
			if i > 0 {
				b.WriteString("  ")
			}
			if i == len(r)-1 {
				b.WriteString(c)
			} else {
				fmt.Fprintf(&b, "%-*s", widths[i], c)
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}

// ConversionJSON is the stable JSON schema for one zone conversion.
type ConversionJSON struct {
	Zone          string `json:"zone"`
	Instant       string `json:"instant"`        // UTC RFC3339 instant
	Local         string `json:"local"`          // formatted wall time
	LocalRFC3339  string `json:"local_rfc3339"`  // wall time as RFC3339 with offset
	Offset        string `json:"offset"`         // +HH:MM
	OffsetSeconds int    `json:"offset_seconds"` //
	Abbrev        string `json:"abbrev"`         //
	IsDST         bool   `json:"is_dst"`         //
	DeltaSeconds  int    `json:"delta_seconds"`  // offset vs source
	DayShift      int    `json:"day_shift"`      // vs source day
	IsSource      bool   `json:"is_source"`      //
}

// RangeJSON is one zone's start/end pair.
type RangeJSON struct {
	Zone  string         `json:"zone"`
	Start ConversionJSON `json:"start"`
	End   ConversionJSON `json:"end"`
}

// GridCellJSON is one grid cell.
type GridCellJSON struct {
	Local    string `json:"local"`
	Hour     int    `json:"hour"`
	DayShift int    `json:"day_shift"`
}

// GridZoneJSON is one grid row.
type GridZoneJSON struct {
	Zone          string         `json:"zone"`
	HasTransition bool           `json:"has_transition"`
	Cells         []GridCellJSON `json:"cells"`
}

// Output is the top-level JSON document.
type Output struct {
	Source      string           `json:"source"`
	SourceTime  string           `json:"source_time"`
	Warnings    []string         `json:"warnings,omitempty"`
	Conversions []ConversionJSON `json:"conversions,omitempty"`
	Grid        []GridZoneJSON   `json:"grid,omitempty"`
	Range       []RangeJSON      `json:"range,omitempty"`
}

// ToConversionJSON maps an engine Conversion to its JSON shape.
func ToConversionJSON(c convert.Conversion, layout string) ConversionJSON {
	return ConversionJSON{
		Zone:          c.Zone,
		Instant:       c.Time.UTC().Format(time.RFC3339),
		Local:         c.Time.Format(layout),
		LocalRFC3339:  c.Time.Format(time.RFC3339),
		Offset:        FormatOffset(c.OffsetSec),
		OffsetSeconds: c.OffsetSec,
		Abbrev:        c.Abbrev,
		IsDST:         c.IsDST,
		DeltaSeconds:  c.DeltaSec,
		DayShift:      c.DayShift,
		IsSource:      c.IsSource,
	}
}

// ToGridJSON maps the engine grid to its JSON shape.
func ToGridJSON(g convert.Grid, layout string) []GridZoneJSON {
	out := make([]GridZoneJSON, 0, len(g.Zones))
	for _, z := range g.Zones {
		gz := GridZoneJSON{Zone: z.Name, HasTransition: z.HasTransition}
		for _, c := range z.Cells {
			gz.Cells = append(gz.Cells, GridCellJSON{
				Local:    c.Time.Format(layout),
				Hour:     c.Time.Hour(),
				DayShift: c.DayShift,
			})
		}
		out = append(out, gz)
	}
	return out
}

// JSON writes the output document as indented JSON.
func JSON(w io.Writer, out Output) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
