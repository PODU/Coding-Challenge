package cron

import (
	"fmt"
	"time"
)

// maxSearchYears is how far NextRun looks before deciding a schedule never
// fires (e.g. "0 0 30 2 *").
const maxSearchYears = 5

// Matches reports whether s fires in the minute containing t.
func (s *Schedule) Matches(t time.Time) bool {
	if s.Minute&(uint64(1)<<uint(t.Minute())) == 0 {
		return false
	}
	if s.Hour&(uint64(1)<<uint(t.Hour())) == 0 {
		return false
	}
	if s.Month&(uint64(1)<<uint(int(t.Month()))) == 0 {
		return false
	}
	return s.dayMatches(t)
}

// dayMatches follows Vixie cron: if both day fields are restricted, either one
// matching is enough; otherwise the restricted one (if any) decides.
func (s *Schedule) dayMatches(t time.Time) bool {
	domMatch := s.Dom&(uint64(1)<<uint(t.Day())) != 0
	dowMatch := s.Dow&(uint64(1)<<uint(int(t.Weekday()))) != 0

	switch {
	case s.DomRestricted && s.DowRestricted:
		return domMatch || dowMatch
	case s.DomRestricted:
		return domMatch
	case s.DowRestricted:
		return dowMatch
	default:
		return true
	}
}

// NextRun returns the first fire time strictly after `after`, in after's
// location. Whole months, days and hours that can't match are skipped at once,
// so sparse schedules are cheap.
func (s *Schedule) NextRun(after time.Time) (time.Time, error) {
	loc := after.Location()
	t := time.Date(after.Year(), after.Month(), after.Day(),
		after.Hour(), after.Minute(), 0, 0, loc).Add(time.Minute)
	limit := t.AddDate(maxSearchYears, 0, 0)

	for t.Before(limit) {
		if s.Month&(uint64(1)<<uint(int(t.Month()))) == 0 {
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, 1, 0)
			continue
		}
		if !s.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
			continue
		}
		if s.Hour&(uint64(1)<<uint(t.Hour())) == 0 {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc).Add(time.Hour)
			continue
		}
		if s.Minute&(uint64(1)<<uint(t.Minute())) == 0 {
			t = t.Add(time.Minute)
			continue
		}
		return t, nil
	}

	return time.Time{}, fmt.Errorf("schedule %q never fires within %d years after %s",
		s.Raw, maxSearchYears, after.Format(time.RFC3339))
}

// NextRuns returns the next n fire times after `after`.
func (s *Schedule) NextRuns(after time.Time, n int) ([]time.Time, error) {
	out := make([]time.Time, 0, n)
	cur := after
	for i := 0; i < n; i++ {
		nx, err := s.NextRun(cur)
		if err != nil {
			return out, err
		}
		out = append(out, nx)
		cur = nx
	}
	return out, nil
}
