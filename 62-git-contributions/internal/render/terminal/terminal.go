// Package terminal prints the calendar and stats to a terminal, in colour or
// as ASCII.
package terminal

import (
	"fmt"
	"io"
	"strings"

	"gitgraph/internal/model"
)

type Options struct {
	Color     bool // ANSI colours; otherwise ASCII characters
	Title     string
	StatsOnly bool
}

// 256-colour background codes for levels 0 to 4, roughly GitHub's greens.
var levelBG = [5]int{235, 22, 28, 34, 40}

var levelRune = [5]rune{'.', ':', '+', '*', '#'}

const (
	margin    = "    " // lines the month header up past the weekday labels
	cellWidth = 2
)

func shortMonth(m int) string {
	names := [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	if m < 1 || m > 12 {
		return "???"
	}
	return names[m-1]
}

var weekdayLabel = [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// cell returns one grid cell, cellWidth columns wide.
func cell(c model.Cell, color bool) string {
	if color {
		return fmt.Sprintf("\x1b[48;5;%dm  \x1b[0m", levelBG[c.Level])
	}
	return string(levelRune[c.Level]) + " "
}

// Render writes the heatmap and stats to w.
func Render(w io.Writer, cal model.Calendar, stats model.Stats, opts Options) error {
	var b strings.Builder

	if opts.Title != "" {
		b.WriteString(opts.Title)
		b.WriteByte('\n')
	}

	if !opts.StatsOnly {
		writeGrid(&b, cal, opts.Color)
		b.WriteByte('\n')
		writeLegend(&b, opts.Color)
		b.WriteByte('\n')
	}

	writeStats(&b, stats)

	_, err := io.WriteString(w, b.String())
	return err
}

func writeGrid(b *strings.Builder, cal model.Calendar, color bool) {
	weeks := cal.Weeks

	header := []byte(strings.Repeat(" ", len(weeks)*cellWidth))
	prevMonth := 0
	for wi, week := range weeks {
		d := week[0].Date
		if int(d.Month) != prevMonth {
			prevMonth = int(d.Month)
			label := shortMonth(int(d.Month))
			pos := wi * cellWidth
			for i := 0; i < len(label) && pos+i < len(header); i++ {
				header[pos+i] = label[i]
			}
		}
	}
	b.WriteString(margin)
	b.WriteString(strings.TrimRight(string(header), " "))
	b.WriteByte('\n')

	for row := 0; row < 7; row++ {
		wd := (int(cal.FirstDay) + row) % 7
		b.WriteString(weekdayLabel[wd] + " ")
		for _, week := range weeks {
			b.WriteString(cell(week[row], color))
		}
		b.WriteByte('\n')
	}
}

func writeLegend(b *strings.Builder, color bool) {
	b.WriteString("Less ")
	for lvl := 0; lvl < 5; lvl++ {
		b.WriteString(cell(model.Cell{Level: lvl}, color))
	}
	b.WriteString(" More\n")
}

func writeStats(b *strings.Builder, s model.Stats) {
	fmt.Fprintf(b, "Total:    %d commits in %d days\n", s.Total, s.Days)
	fmt.Fprintf(b, "Active:   %d days\n", s.ActiveDays)
	fmt.Fprintf(b, "Max/day:  %d\n", s.Max)
	fmt.Fprintf(b, "Streaks:  current %d, longest %d\n", s.CurrentStreak, s.LongestStreak)
	if s.Total > 0 {
		fmt.Fprintf(b, "Busiest:  %s (%d)\n", s.Busiest, s.Max)
	} else {
		b.WriteString("Busiest:  -\n")
	}
}

// RenderLeaderboard prints the leaderboard table.
func RenderLeaderboard(w io.Writer, contribs []model.Contributor) error {
	var b strings.Builder
	b.WriteString("\nLeaderboard\n")
	for i, c := range contribs {
		who := c.Name
		if c.Email != "" {
			who = fmt.Sprintf("%s <%s>", c.Name, c.Email)
		}
		fmt.Fprintf(&b, "%2d. %-30s %4d commits, %d active days\n", i+1, who, c.Total, c.Active)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
