// Package store persists posts. Rather than plain CRUD, Store has one method
// per state change, each done in a transaction, so rules like "a due post is
// claimed exactly once" are enforced in one place.
package store

import (
	"context"
	"errors"
	"time"

	"socialmedia/internal/model"
)

var (
	ErrNotFound = errors.New("store: post not found")
	// ErrNotEditable: the post has already been queued.
	ErrNotEditable = errors.New("store: post not editable")
	// ErrNotClaimable: BeginPublish found the post in some state other than
	// queued, e.g. another worker got it first.
	ErrNotClaimable = errors.New("store: post not claimable")
)

// ListFilter selects posts for List. Empty fields match everything.
type ListFilter struct {
	Status string
	Target string
	Author string
	Limit  int
	Offset int
}

// EditFields are the changes for Edit; nil fields are left alone.
type EditFields struct {
	Body        *string
	Visibility  *model.Visibility
	ScheduledAt *time.Time
}

type Store interface {
	// Create saves a new post, which must already have its ID and timestamps.
	Create(ctx context.Context, p *model.Post) error

	Get(ctx context.Context, id string) (*model.Post, error)

	// List returns one page of matching posts, newest first, and the total
	// number of matches.
	List(ctx context.Context, f ListFilter) ([]*model.Post, int, error)

	// ClaimDue moves up to limit due posts from scheduled to queued and
	// returns them. Concurrent callers never get the same post.
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]*model.Post, error)

	// PublishNow queues a draft or scheduled post immediately.
	PublishNow(ctx context.Context, id string, now time.Time) (*model.Post, error)

	// BeginPublish moves a queued post to publishing and counts the attempt.
	BeginPublish(ctx context.Context, id string, now time.Time) (*model.Post, error)

	FinishPublish(ctx context.Context, id, externalID string, now time.Time) (*model.Post, error)

	// RetryPublish puts a publishing post back in the queue and records why it
	// failed.
	RetryPublish(ctx context.Context, id, errMsg string, now time.Time) (*model.Post, error)

	// FailPublish gives up on a post for good.
	FailPublish(ctx context.Context, id, errMsg string, now time.Time) (*model.Post, error)

	// Edit changes a draft or scheduled post. Its status follows the new
	// scheduled_at.
	Edit(ctx context.Context, id string, f EditFields, now time.Time) (*model.Post, error)

	Cancel(ctx context.Context, id string, now time.Time) (*model.Post, error)

	Ping(ctx context.Context) error

	Close() error
}
