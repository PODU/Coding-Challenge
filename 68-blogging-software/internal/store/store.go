// Package store saves posts and comments, either as JSON files or in SQLite.
package store

import (
	"context"
	"errors"
	"time"

	"blog/internal/blog"
)

// ErrNotFound is returned when a requested post does not exist.
var ErrNotFound = errors.New("not found")

// Order is the sort order for List.
type Order int

const (
	NewestFirst Order = iota
	OldestFirst
)

// ListOpts filters and orders List results.
type ListOpts struct {
	// Status filters by status unless empty.
	Status blog.Status
	// PublishedBefore, if set, drops posts not published by that time, which
	// hides scheduled posts.
	PublishedBefore *time.Time
	Order           Order
	// Limit 0 means no limit.
	Limit int
}

// Store saves posts and comments.
type Store interface {
	Create(ctx context.Context, p *blog.Post) (int64, error)
	Update(ctx context.Context, p *blog.Post) error
	Get(ctx context.Context, id int64) (*blog.Post, error)
	GetBySlug(ctx context.Context, slug string) (*blog.Post, error)
	List(ctx context.Context, opts ListOpts) ([]*blog.Post, error)
	AddComment(ctx context.Context, c *blog.Comment) error
	ListComments(ctx context.Context, postID int64) ([]*blog.Comment, error)
	Close() error
}

// effectiveDate is what posts are sorted by: PublishedAt, or CreatedAt for
// drafts.
func effectiveDate(p *blog.Post) time.Time {
	if p.PublishedAt != nil {
		return *p.PublishedAt
	}
	return p.CreatedAt
}

// matches applies the filters in opts (not the order or limit).
func matches(p *blog.Post, opts ListOpts) bool {
	if opts.Status != "" && p.Status != opts.Status {
		return false
	}
	if opts.PublishedBefore != nil {
		if p.PublishedAt == nil || p.PublishedAt.After(*opts.PublishedBefore) {
			return false
		}
	}
	return true
}
