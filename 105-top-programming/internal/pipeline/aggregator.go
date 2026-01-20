// Package pipeline performs the concurrent fan-out, dedup and normalization of
// stories pulled from a source.Source.
package pipeline

import (
	"context"
	"log/slog"
	"sort"
	"sync"

	"github.com/codingchallenges/topstories/internal/model"
	"github.com/codingchallenges/topstories/internal/source"
)

// DefaultWorkers bounds the fan-out pool when an invalid value is supplied.
const DefaultWorkers = 16

// Aggregator fans out item fetches across a bounded worker pool, dedups by ID
// (highest score wins) and normalizes results.
type Aggregator struct {
	src       source.Source
	workers   int
	maxPerCat int // 0 = no limit
	log       *slog.Logger
}

// Option configures an Aggregator.
type Option func(*Aggregator)

// WithMaxPerCategory caps how many IDs are fetched per category (0 = all).
func WithMaxPerCategory(n int) Option { return func(a *Aggregator) { a.maxPerCat = n } }

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) Option { return func(a *Aggregator) { a.log = l } }

// New builds an Aggregator. workers <= 0 falls back to DefaultWorkers.
func New(src source.Source, workers int, opts ...Option) *Aggregator {
	if workers <= 0 {
		workers = DefaultWorkers
	}
	a := &Aggregator{src: src, workers: workers, log: slog.Default()}
	for _, o := range opts {
		o(a)
	}
	return a
}

// idTask pairs an ID with the category it was discovered under.
type idTask struct {
	id  int64
	cat source.Category
}

// Collect fans out over the given categories and returns deduped stories.
//
// One failing item never fails the batch: per-item errors are logged and the
// partial result is returned. Context cancellation stops in-flight work promptly.
func (a *Aggregator) Collect(ctx context.Context, cats ...source.Category) ([]model.Story, error) {
	// An ID listed under several categories is only fetched once, under the
	// first category it appeared in.
	tasks := make([]idTask, 0, 512)
	queued := make(map[int64]struct{})
	for _, cat := range cats {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ids, err := a.src.StoryIDs(ctx, cat)
		if err != nil {
			a.log.Warn("story ids failed", "category", cat, "err", err)
			continue
		}
		if a.maxPerCat > 0 && len(ids) > a.maxPerCat {
			ids = ids[:a.maxPerCat]
		}
		for _, id := range ids {
			if _, ok := queued[id]; ok {
				continue
			}
			queued[id] = struct{}{}
			tasks = append(tasks, idTask{id: id, cat: cat})
		}
	}

	taskCh := make(chan idTask)
	var (
		mu   sync.Mutex
		byID = make(map[int64]model.Story, len(tasks))
		wg   sync.WaitGroup
	)

	worker := func() {
		defer wg.Done()
		for t := range taskCh {
			st, err := a.src.Fetch(ctx, t.id)
			if err != nil {
				if source.IsSkip(err) {
					a.log.Debug("item skipped", "id", t.id, "reason", err)
				} else if ctx.Err() != nil {
					return
				} else {
					a.log.Warn("fetch failed", "id", t.id, "err", err)
				}
				continue
			}
			st.Category = string(t.cat)
			mu.Lock()
			if prev, ok := byID[st.ID]; !ok || st.Score > prev.Score {
				byID[st.ID] = st
			}
			mu.Unlock()
		}
	}

	wg.Add(a.workers)
	for i := 0; i < a.workers; i++ {
		go worker()
	}

feed:
	for _, t := range tasks {
		select {
		case <-ctx.Done():
			break feed
		case taskCh <- t:
		}
	}
	close(taskCh)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Map iteration order is random, so sort for deterministic output.
	out := make([]model.Story, 0, len(byID))
	for _, s := range byID {
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
