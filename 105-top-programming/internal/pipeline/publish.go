package pipeline

import (
	"context"
	"log/slog"

	"github.com/codingchallenges/topstories/internal/broker"
	"github.com/codingchallenges/topstories/internal/model"
)

// Deduper tracks already-published story IDs so the producer loop does not
// re-emit unchanged stories. It is safe for single-goroutine producer use.
type Deduper struct {
	seen map[int64]int // id -> last published score
}

// NewDeduper builds an empty Deduper.
func NewDeduper() *Deduper { return &Deduper{seen: make(map[int64]int)} }

// ShouldPublish reports whether s is new or has a higher score than last seen,
// recording it as published when true.
func (d *Deduper) ShouldPublish(s model.Story) bool {
	prev, ok := d.seen[s.ID]
	if ok && s.Score <= prev {
		return false
	}
	d.seen[s.ID] = s.Score
	return true
}

// PublishAll publishes the deduped subset of stories to p, returning the number
// actually published. Per-story publish errors are logged and skipped.
func PublishAll(ctx context.Context, p broker.Producer, d *Deduper, stories []model.Story, log *slog.Logger) int {
	published := 0
	for _, s := range stories {
		if err := ctx.Err(); err != nil {
			return published
		}
		if !d.ShouldPublish(s) {
			continue
		}
		if err := p.Publish(ctx, s); err != nil {
			log.Warn("publish failed", "id", s.ID, "err", err)
			continue
		}
		published++
	}
	return published
}
