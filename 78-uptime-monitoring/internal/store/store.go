// Package store persists monitors and their check results, in memory or in
// SQLite (pure-Go driver, no cgo).
package store

import (
	"context"
	"time"

	"github.com/example/uptime/internal/monitor"
)

// Store is implemented by Memory and SQLite.
type Store interface {
	// SaveResult appends a single time-series data point.
	SaveResult(ctx context.Context, r monitor.CheckResult) error
	// History returns results for a monitor within [from, to], ordered by time.
	History(ctx context.Context, monitorID string, from, to time.Time) ([]monitor.CheckResult, error)
	// LatestStatus returns the most recent status for a monitor, or
	// StatusUnknown if there are no results.
	LatestStatus(ctx context.Context, monitorID string) (monitor.Status, error)
	// UpsertMonitor inserts or updates a monitor's configuration.
	UpsertMonitor(ctx context.Context, m monitor.Monitor) error
	// DeleteMonitor removes a monitor and its results.
	DeleteMonitor(ctx context.Context, monitorID string) error
	// ListMonitors returns all stored monitors.
	ListMonitors(ctx context.Context) ([]monitor.Monitor, error)
	// Close releases any underlying resources.
	Close() error
}
