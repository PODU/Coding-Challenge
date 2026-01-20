// Package model defines the normalized Story type shared across the pipeline.
package model

import (
	"errors"
	"time"
)

// Story is a normalized programming story from any source.
type Story struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Score     int       `json:"score"`
	Author    string    `json:"author"`    // from HN "by"
	Timestamp time.Time `json:"timestamp"` // from HN "time" (unix seconds)
	Category  string    `json:"category"`  // top|new|best|ask|show
	Type      string    `json:"type"`      // story|job|poll
	Source    string    `json:"source"`    // "hackernews", "reddit", ...
}

// Validate reports whether the story carries the minimum required fields.
func (s Story) Validate() error {
	if s.ID <= 0 {
		return errors.New("story: id must be positive")
	}
	if s.Title == "" {
		return errors.New("story: title must not be empty")
	}
	if s.Source == "" {
		return errors.New("story: source must not be empty")
	}
	return nil
}
