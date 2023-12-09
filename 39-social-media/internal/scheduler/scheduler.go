// Package scheduler polls the store for posts that have come due and queues
// them for publishing.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"socialmedia/internal/clock"
	"socialmedia/internal/queue"
	"socialmedia/internal/store"
)

type Scheduler struct {
	store    store.Store
	queue    queue.Queue
	clock    clock.Clock
	interval time.Duration
	batch    int
	log      *slog.Logger
}

// Config for New. Store, Queue and Clock are required.
type Config struct {
	Store    store.Store
	Queue    queue.Queue
	Clock    clock.Clock
	Interval time.Duration // default 5s
	Batch    int           // posts claimed per tick, default 100
	Logger   *slog.Logger
}

func New(cfg Config) *Scheduler {
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Batch <= 0 {
		cfg.Batch = 100
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Scheduler{
		store:    cfg.Store,
		queue:    cfg.Queue,
		clock:    cfg.Clock,
		interval: cfg.Interval,
		batch:    cfg.Batch,
		log:      cfg.Logger,
	}
}

// Tick claims the posts that are due now, queues them, and returns how many
// it queued.
func (s *Scheduler) Tick(ctx context.Context) (int, error) {
	now := s.clock.Now()
	claimed, err := s.store.ClaimDue(ctx, now, s.batch)
	if err != nil {
		return 0, err
	}
	enqueued := 0
	for _, p := range claimed {
		if err := s.queue.Enqueue(ctx, queue.Message{PostID: p.ID}); err != nil {
			return enqueued, err
		}
		enqueued++
		s.log.Debug("dispatched scheduled post", "post_id", p.ID)
	}
	return enqueued, nil
}

// Run calls Tick every interval until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	t := time.NewTicker(s.interval)
	defer t.Stop()

	// Don't make posts that are already due wait for the first tick.
	if _, err := s.Tick(ctx); err != nil && ctx.Err() == nil {
		s.log.Error("scheduler tick failed", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if _, err := s.Tick(ctx); err != nil && ctx.Err() == nil {
				s.log.Error("scheduler tick failed", "err", err)
			}
		}
	}
}
