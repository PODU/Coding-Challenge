// Package sweep periodically deletes expired pastes.
package sweep

import (
	"context"
	"log/slog"
	"time"

	"pastebin/internal/clock"
	"pastebin/internal/store"
)

// Sweeper deletes expired pastes.
type Sweeper struct {
	store    store.Store
	clk      clock.Clock
	interval time.Duration
	log      *slog.Logger
}

// New returns a Sweeper. A nil logger means slog.Default.
func New(s store.Store, clk clock.Clock, interval time.Duration, log *slog.Logger) *Sweeper {
	if log == nil {
		log = slog.Default()
	}
	if clk == nil {
		clk = clock.Real{}
	}
	return &Sweeper{store: s, clk: clk, interval: interval, log: log}
}

// RunOnce sweeps once and returns the number of pastes removed.
func (s *Sweeper) RunOnce(ctx context.Context) (int, error) {
	return s.store.DeleteExpired(ctx, s.clk.Now())
}

// Run sweeps immediately and then every interval until ctx is done.
func (s *Sweeper) Run(ctx context.Context) {
	if s.interval <= 0 {
		s.interval = time.Minute
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	sweep := func() {
		n, err := s.RunOnce(ctx)
		if err != nil {
			if ctx.Err() == nil {
				s.log.Error("sweep failed", "error", err)
			}
			return
		}
		if n > 0 {
			s.log.Info("swept expired pastes", "count", n)
		}
	}

	sweep()
	for {
		select {
		case <-ctx.Done():
			s.log.Info("sweeper stopped")
			return
		case <-ticker.C:
			sweep()
		}
	}
}
