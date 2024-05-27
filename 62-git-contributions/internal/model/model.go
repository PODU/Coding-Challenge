// Package model holds the types shared across gitgraph.
package model

import (
	"fmt"
	"time"
)

// Date is a calendar date with no time zone. Arithmetic is done on UTC
// midnight so DST changes can't shift a day.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the civil date of t in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// DateIn returns the civil date of t after converting it into loc.
func DateIn(t time.Time, loc *time.Location) Date {
	return DateOf(t.In(loc))
}

// Time returns midnight of d in loc.
func (d Date) Time(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

func (d Date) utc() time.Time { return d.Time(time.UTC) }

// AddDays returns the date n days after d (n may be negative).
func (d Date) AddDays(n int) Date {
	return DateOf(d.utc().AddDate(0, 0, n))
}

func (d Date) Weekday() time.Weekday { return d.utc().Weekday() }

// DaysSince returns d minus o in days.
func (d Date) DaysSince(o Date) int {
	return int(d.utc().Sub(o.utc()).Hours() / 24)
}

func (d Date) Before(o Date) bool { return d.utc().Before(o.utc()) }

func (d Date) After(o Date) bool { return d.utc().After(o.utc()) }

func (d Date) Equal(o Date) bool { return d == o }

// String formats d as YYYY-MM-DD.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// ParseDate parses a YYYY-MM-DD string.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD): %w", s, err)
	}
	return DateOf(t), nil
}

type Commit struct {
	SHA         string
	AuthorName  string
	AuthorEmail string
	When        time.Time
}

type DayCount struct {
	Date  Date
	Count int
}

// Cell is one day in the calendar grid.
type Cell struct {
	Date    Date
	Count   int
	Level   int  // shade, 0 to 4
	InRange bool // false for padding outside Start..End
}

// Calendar is a grid of weeks. Weeks[w][r] is week w, weekday row r, where
// row 0 is FirstDay.
type Calendar struct {
	Start    Date
	End      Date
	FirstDay time.Weekday
	Weeks    [][7]Cell
}

type Stats struct {
	Total         int
	ActiveDays    int
	Max           int
	CurrentStreak int
	LongestStreak int
	Busiest       Date
	Days          int // length of the date range
}

// Contributor is one leaderboard row.
type Contributor struct {
	Name   string
	Email  string
	Total  int
	Active int
}
