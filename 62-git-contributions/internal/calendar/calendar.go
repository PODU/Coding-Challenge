// Package calendar lays day counts out as a grid with one column per week.
package calendar

import (
	"time"

	"gitgraph/internal/model"
)

// alignBack returns the start of d's week.
func alignBack(d model.Date, firstDay time.Weekday) model.Date {
	delta := (int(d.Weekday()) - int(firstDay) + 7) % 7
	return d.AddDays(-delta)
}

// Level maps count to a shade from 0 to 4: 0 for no commits, otherwise
// ceil(4*count/max).
func Level(count, max int) int {
	if count <= 0 || max <= 0 {
		return 0
	}
	lvl := (count*4 + max - 1) / max
	if lvl > 4 {
		lvl = 4
	}
	return lvl
}

// Build lays out start..end as week columns starting on firstDay. Days before
// start or after end in the first and last weeks are filled with InRange=false
// cells so every column has seven rows.
func Build(counts map[model.Date]int, start, end model.Date, firstDay time.Weekday) model.Calendar {
	cal := model.Calendar{Start: start, End: end, FirstDay: firstDay}
	if end.Before(start) {
		return cal
	}

	max := 0
	for d, n := range counts {
		if d.Before(start) || d.After(end) {
			continue
		}
		if n > max {
			max = n
		}
	}

	gridStart := alignBack(start, firstDay)

	var week [7]model.Cell
	for d := gridStart; !d.After(end); d = d.AddDays(1) {
		row := (int(d.Weekday()) - int(firstDay) + 7) % 7
		inRange := !d.Before(start) && !d.After(end)
		cnt := 0
		if inRange {
			cnt = counts[d]
		}
		week[row] = model.Cell{
			Date:    d,
			Count:   cnt,
			Level:   Level(cnt, max),
			InRange: inRange,
		}
		if row == 6 {
			cal.Weeks = append(cal.Weeks, week)
			week = [7]model.Cell{}
		}
	}
	// Fill out a partial last week. Padding cells keep their real dates.
	lastRow := (int(end.Weekday()) - int(firstDay) + 7) % 7
	if lastRow != 6 {
		for r := lastRow + 1; r < 7; r++ {
			week[r] = model.Cell{Date: end.AddDays(r - lastRow)}
		}
		cal.Weeks = append(cal.Weeks, week)
	}
	return cal
}
