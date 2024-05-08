// Package store has the paste storage interface and its in-memory and SQLite
// implementations.
package store

import (
	"context"
	"time"

	"pastebin/internal/paste"
)

// These are the paste package's errors, so the service can check for them
// without importing this package.
var (
	// ErrNotFound is returned by Get/Delete when no paste with the id exists.
	ErrNotFound = paste.ErrNotFound
	// ErrConflict is returned by Create when the id already exists.
	ErrConflict = paste.ErrConflict
)

// Store implementations must be safe for concurrent use.
type Store interface {
	// Create persists a new paste, returning ErrConflict if the id exists.
	Create(ctx context.Context, p paste.Paste) error
	// Get returns expired pastes too; the service decides what expired means.
	Get(ctx context.Context, id string) (paste.Paste, error)
	// Delete removes the paste by id, returning ErrNotFound if absent.
	Delete(ctx context.Context, id string) error
	// DeleteExpired removes pastes with expiry <= now and returns the count.
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
	// Ping is used by /healthz.
	Ping(ctx context.Context) error
	Close() error
}
