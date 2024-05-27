// Package aggregate filters commits, counts them per day and computes the
// summary statistics.
package aggregate

import (
	"sort"
	"strings"
	"time"

	"gitgraph/internal/model"
)

// Dedup drops repeated SHAs, keeping the first. The same commit can show up
// more than once when several clones of a repo are scanned.
func Dedup(commits []model.Commit) []model.Commit {
	seen := make(map[string]struct{}, len(commits))
	out := commits[:0:0]
	for _, c := range commits {
		if c.SHA != "" {
			if _, ok := seen[c.SHA]; ok {
				continue
			}
			seen[c.SHA] = struct{}{}
		}
		out = append(out, c)
	}
	return out
}

// MatchesAuthor reports whether the author name or email contains substr,
// ignoring case. An empty substr matches everything.
func MatchesAuthor(c model.Commit, substr string) bool {
	if substr == "" {
		return true
	}
	s := strings.ToLower(substr)
	return strings.Contains(strings.ToLower(c.AuthorName), s) ||
		strings.Contains(strings.ToLower(c.AuthorEmail), s)
}

// Filter returns the commits whose author matches substr (see MatchesAuthor).
func Filter(commits []model.Commit, substr string) []model.Commit {
	if substr == "" {
		return commits
	}
	out := make([]model.Commit, 0, len(commits))
	for _, c := range commits {
		if MatchesAuthor(c, substr) {
			out = append(out, c)
		}
	}
	return out
}

// Bucket counts commits per day, where the day is taken in loc (UTC if nil).
// Only days from start to end inclusive are counted.
func Bucket(commits []model.Commit, loc *time.Location, start, end model.Date) map[model.Date]int {
	if loc == nil {
		loc = time.UTC
	}
	counts := make(map[model.Date]int)
	for _, c := range commits {
		d := model.DateIn(c.When, loc)
		if d.Before(start) || d.After(end) {
			continue
		}
		counts[d]++
	}
	return counts
}

// DayCounts returns counts as a slice sorted by date.
func DayCounts(counts map[model.Date]int) []model.DayCount {
	out := make([]model.DayCount, 0, len(counts))
	for d, n := range counts {
		out = append(out, model.DayCount{Date: d, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

// ComputeStats summarises counts between start and end inclusive. The current
// streak is the run of active days ending on end. Ties for busiest day go to
// the earliest date.
func ComputeStats(counts map[model.Date]int, start, end model.Date) model.Stats {
	st := model.Stats{Days: end.DaysSince(start) + 1}
	if st.Days < 0 {
		st.Days = 0
	}

	for d, n := range counts {
		if n <= 0 || d.Before(start) || d.After(end) {
			continue
		}
		st.Total += n
		st.ActiveDays++
		if n > st.Max {
			st.Max = n
			st.Busiest = d
		} else if n == st.Max && d.Before(st.Busiest) {
			st.Busiest = d
		}
	}

	run := 0
	for d := start; !d.After(end); d = d.AddDays(1) {
		if counts[d] > 0 {
			run++
			if run > st.LongestStreak {
				st.LongestStreak = run
			}
		} else {
			run = 0
		}
	}

	for d := end; !d.Before(start) && counts[d] > 0; d = d.AddDays(-1) {
		st.CurrentStreak++
	}

	return st
}

// Leaderboard counts commits per author between start and end, most commits
// first. Authors are identified by email, or by name if there is no email.
func Leaderboard(commits []model.Commit, loc *time.Location, start, end model.Date) []model.Contributor {
	if loc == nil {
		loc = time.UTC
	}
	type acc struct {
		c    model.Contributor
		days map[model.Date]struct{}
	}
	byKey := make(map[string]*acc)
	for _, c := range commits {
		d := model.DateIn(c.When, loc)
		if d.Before(start) || d.After(end) {
			continue
		}
		key := strings.ToLower(c.AuthorEmail)
		if key == "" {
			key = strings.ToLower(c.AuthorName)
		}
		a := byKey[key]
		if a == nil {
			a = &acc{c: model.Contributor{Name: c.AuthorName, Email: c.AuthorEmail}, days: map[model.Date]struct{}{}}
			byKey[key] = a
		}
		a.c.Total++
		a.days[d] = struct{}{}
	}
	out := make([]model.Contributor, 0, len(byKey))
	for _, a := range byKey {
		a.c.Active = len(a.days)
		out = append(out, a.c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Name < out[j].Name
	})
	return out
}
