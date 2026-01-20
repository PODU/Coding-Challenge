// Package ranking provides pure (I/O-free) filtering and ranking of stories.
package ranking

import (
	"sort"
	"strings"
	"time"

	"github.com/codingchallenges/topstories/internal/model"
)

// Filter describes how to narrow a slice of stories. Zero values mean "no
// constraint" for that field.
type Filter struct {
	MinScore int
	Since    time.Time // zero = no lower bound
	Until    time.Time // zero = no upper bound
	Types    []string  // empty = any
	Keywords []string  // case-insensitive title match, empty = any
}

// Apply returns the subset of in matching f, preserving input order.
func Apply(in []model.Story, f Filter) []model.Story {
	out := make([]model.Story, 0, len(in))
	for _, s := range in {
		if matches(s, f) {
			out = append(out, s)
		}
	}
	return out
}

func matches(s model.Story, f Filter) bool {
	if s.Score < f.MinScore {
		return false
	}
	if !f.Since.IsZero() && s.Timestamp.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && s.Timestamp.After(f.Until) {
		return false
	}
	if len(f.Types) > 0 && !containsFold(f.Types, s.Type) {
		return false
	}
	if len(f.Keywords) > 0 {
		title := strings.ToLower(s.Title)
		found := false
		for _, kw := range f.Keywords {
			if kw == "" {
				continue
			}
			if strings.Contains(title, strings.ToLower(kw)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func containsFold(set []string, v string) bool {
	for _, s := range set {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

// TopN returns the n highest-scored stories (score desc, then ID asc for
// stable ties). It does not mutate in. n <= 0 or n > len returns all.
func TopN(in []model.Story, n int) []model.Story {
	out := make([]model.Story, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	if n > 0 && n < len(out) {
		out = out[:n]
	}
	return out
}
