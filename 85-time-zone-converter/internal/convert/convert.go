// Package convert shows an instant in each zone, with its offset, whether DST
// is on, and whether the date differs from the source. It also builds the grid
// and range views.
package convert

import (
	"sort"
	"time"

	"tzc/internal/zones"
)

// Conversion is one instant in one zone.
type Conversion struct {
	Zone      string // canonical IANA name
	Loc       *time.Location
	Time      time.Time // in Loc
	Abbrev    string    // e.g. EDT, or +0545 where there is no abbreviation
	OffsetSec int
	IsDST     bool
	DeltaSec  int  // offset minus the source zone's offset
	DayShift  int  // this zone's date minus the source's date, in days
	IsSource  bool // the source zone's own row
}

// Convert shows instant in each of zs, compared against source.
func Convert(instant time.Time, zs []zones.Zone, source *time.Location) []Conversion {
	srcWall := instant.In(source)
	_, srcOff := srcWall.Zone()

	out := make([]Conversion, 0, len(zs))
	for _, z := range zs {
		w := instant.In(z.Loc)
		abbr, off := w.Zone()
		out = append(out, Conversion{
			Zone:      z.Name,
			Loc:       z.Loc,
			Time:      w,
			Abbrev:    abbr,
			OffsetSec: off,
			IsDST:     isDST(w),
			DeltaSec:  off - srcOff,
			DayShift:  dayShift(w, srcWall),
			IsSource:  sameZone(z.Loc, source),
		})
	}
	return out
}

// SortByOffset orders conversions west to east, keeping ties in order.
func SortByOffset(cs []Conversion) {
	sort.SliceStable(cs, func(i, j int) bool {
		return cs[i].DeltaSec < cs[j].DeltaSec
	})
}

// Grid is the 24-hour planner: one row per zone, one column per hour.
type Grid struct {
	Base  time.Time // 00:00 in the first zone
	Zones []GridZone
}

type GridZone struct {
	Name          string
	Cells         []GridCell
	HasTransition bool // a DST change happens during these 24 hours
}

type GridCell struct {
	Time     time.Time
	DayShift int // relative to Base's day
}

// BuildGrid builds the 24 hours starting at midnight, in the first zone, of
// the day containing instant.
func BuildGrid(instant time.Time, zs []zones.Zone) Grid {
	if len(zs) == 0 {
		return Grid{}
	}
	first := zs[0].Loc
	fw := instant.In(first)
	base := time.Date(fw.Year(), fw.Month(), fw.Day(), 0, 0, 0, 0, first)
	baseY, baseM, baseD := base.Date()
	baseDay := time.Date(baseY, baseM, baseD, 0, 0, 0, 0, time.UTC)

	g := Grid{Base: base}
	for _, z := range zs {
		row := GridZone{Name: z.Name}
		_, prevOff := base.In(z.Loc).Zone()
		for h := 0; h < 24; h++ {
			t := base.Add(time.Duration(h) * time.Hour).In(z.Loc)
			_, off := t.Zone()
			if off != prevOff {
				row.HasTransition = true
			}
			prevOff = off
			ty, tm, td := t.Date()
			shift := int(time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC).Sub(baseDay).Hours()) / 24
			row.Cells = append(row.Cells, GridCell{Time: t, DayShift: shift})
		}
		g.Zones = append(g.Zones, row)
	}
	return g
}

type RangeRow struct {
	Start Conversion
	End   Conversion
}

func BuildRange(start, end time.Time, zs []zones.Zone, source *time.Location, sortZones bool) []RangeRow {
	starts := Convert(start, zs, source)
	ends := Convert(end, zs, source)
	if sortZones {
		SortByOffset(starts)
		SortByOffset(ends)
	}
	rows := make([]RangeRow, len(starts))
	for i := range starts {
		rows[i] = RangeRow{Start: starts[i], End: ends[i]}
	}
	return rows
}

// isDST guesses whether DST is on at t. The standard offset is taken to be the
// smaller of the offsets on 1 January and 1 July, which works for both
// hemispheres.
func isDST(t time.Time) bool {
	loc := t.Location()
	_, offJan := time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, loc).Zone()
	_, offJul := time.Date(t.Year(), time.July, 1, 0, 0, 0, 0, loc).Zone()
	_, off := t.Zone()
	return off != min(offJan, offJul)
}

func dayShift(target, source time.Time) int {
	ty, tm, td := target.Date()
	sy, sm, sd := source.Date()
	a := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	b := time.Date(sy, sm, sd, 0, 0, 0, 0, time.UTC)
	return int(a.Sub(b).Hours()) / 24
}

func sameZone(a, b *time.Location) bool {
	return a == b || a.String() == b.String()
}
