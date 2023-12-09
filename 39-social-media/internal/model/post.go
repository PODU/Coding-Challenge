// Package model defines Post and the checks that apply whatever network it is
// going to.
package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusScheduled  Status = "scheduled"
	StatusQueued     Status = "queued"
	StatusPublishing Status = "publishing"
	StatusPublished  Status = "published"
	StatusFailed     Status = "failed"
	StatusCanceled   Status = "canceled"
)

// Visibility is the audience; each provider maps it to its own setting.
type Visibility string

const (
	VisibilityPublic      Visibility = "public"
	VisibilityConnections Visibility = "connections"
	VisibilityPrivate     Visibility = "private"
)

// ErrValidation wraps every bad-input error; the API turns it into a 400.
var ErrValidation = errors.New("validation error")

func validationErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

// Post is a post to one network. Times are in UTC. A nil ScheduledAt means
// publish straight away.
type Post struct {
	ID          string     `json:"id"`
	Author      string     `json:"author"`
	Body        string     `json:"body"`
	Visibility  Visibility `json:"visibility"`
	Target      string     `json:"target"`
	Status      Status     `json:"status"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	PublishedAt *time.Time `json:"published_at"`
	ExternalID  string     `json:"external_id"`
	Error       string     `json:"error,omitempty"`
	Attempts    int        `json:"attempts"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func ValidVisibility(v Visibility) bool {
	switch v {
	case VisibilityPublic, VisibilityConnections, VisibilityPrivate:
		return true
	default:
		return false
	}
}

// Editable reports whether a post can still be edited or canceled, which is
// only true before it has been queued.
func (s Status) Editable() bool {
	return s == StatusDraft || s == StatusScheduled
}

// Validate checks a new post. Limits that depend on the network, such as body
// length, are checked by the Provider instead. A scheduled time must be after
// now.
func (p *Post) Validate(now time.Time) error {
	if strings.TrimSpace(p.Author) == "" {
		return validationErr("author is required")
	}
	if strings.TrimSpace(p.Body) == "" {
		return validationErr("body is required")
	}
	if strings.TrimSpace(p.Target) == "" {
		return validationErr("target is required")
	}
	if !ValidVisibility(p.Visibility) {
		return validationErr("visibility must be one of public, connections, private")
	}
	if p.ScheduledAt != nil && !p.ScheduledAt.After(now) {
		return validationErr("scheduled_at must be in the future")
	}
	return nil
}
