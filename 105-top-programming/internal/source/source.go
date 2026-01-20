// Package source defines the network boundary for story providers.
//
// Every implementation that crosses a network boundary (Hacker News, Reddit,
// ...) satisfies Source so that the aggregation core can be tested offline
// against fakes/fixtures.
package source

import (
	"context"

	"github.com/codingchallenges/topstories/internal/model"
)

// Category identifies a Hacker News story listing.
type Category string

const (
	CatTop  Category = "top"
	CatNew  Category = "new"
	CatBest Category = "best"
	CatAsk  Category = "ask"
	CatShow Category = "show"
)

// ParseCategory converts a string into a Category, reporting whether it is known.
func ParseCategory(s string) (Category, bool) {
	switch Category(s) {
	case CatTop, CatNew, CatBest, CatAsk, CatShow:
		return Category(s), true
	default:
		return "", false
	}
}

// Source is the network boundary mocked in tests.
type Source interface {
	// Name identifies the provider, e.g. "hackernews".
	Name() string
	// StoryIDs returns the ordered IDs for a category.
	StoryIDs(ctx context.Context, c Category) ([]int64, error)
	// Fetch returns the full normalized story for one ID.
	Fetch(ctx context.Context, id int64) (model.Story, error)
}

type skipError struct{ reason string }

func (e skipError) Error() string { return "skip: " + e.reason }

// Skip returns an error meaning "drop this item" (e.g. deleted or dead) rather
// than a real failure.
func Skip(reason string) error { return skipError{reason: reason} }

// IsSkip reports whether err is a Skip sentinel.
func IsSkip(err error) bool {
	_, ok := err.(skipError)
	return ok
}
