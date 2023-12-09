// Package publisher takes posts off the queue and publishes them, retrying
// failures a limited number of times.
package publisher

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"socialmedia/internal/clock"
	"socialmedia/internal/model"
	"socialmedia/internal/provider"
	"socialmedia/internal/queue"
	"socialmedia/internal/store"
)

// Config for New. Store, Queue and Registry are required.
type Config struct {
	Store    store.Store
	Queue    queue.Queue
	Registry *provider.Registry
	Clock    clock.Clock

	Workers     int // default 4
	MaxAttempts int // default 3
	// Backoff is the wait before retrying after attempt n (from 1). The
	// default is DefaultBackoff; tests use one that doesn't wait.
	Backoff func(attempt int) time.Duration

	Logger *slog.Logger
}

type Publisher struct {
	store       store.Store
	queue       queue.Queue
	registry    *provider.Registry
	clock       clock.Clock
	workers     int
	maxAttempts int
	backoff     func(int) time.Duration
	log         *slog.Logger

	wg sync.WaitGroup
}

// DefaultBackoff waits a random time up to 100ms * 2^(attempt-1), capped at 30s.
func DefaultBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	base := 100 * time.Millisecond
	max := 30 * time.Second
	d := base << (attempt - 1)
	if d > max || d <= 0 {
		d = max
	}
	return time.Duration(rand.Int63n(int64(d) + 1))
}

func New(cfg Config) *Publisher {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.Backoff == nil {
		cfg.Backoff = DefaultBackoff
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Publisher{
		store:       cfg.Store,
		queue:       cfg.Queue,
		registry:    cfg.Registry,
		clock:       cfg.Clock,
		workers:     cfg.Workers,
		maxAttempts: cfg.MaxAttempts,
		backoff:     cfg.Backoff,
		log:         cfg.Logger,
	}
}

// Start starts the workers. They stop when ctx is cancelled or the queue is
// closed and empty; Wait waits for that.
func (p *Publisher) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
}

func (p *Publisher) Wait() { p.wg.Wait() }

func (p *Publisher) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	for {
		msg, err := p.queue.Dequeue(ctx)
		if err != nil {
			return
		}
		p.handle(ctx, msg)
	}
}

// handle publishes one post. Errors are only logged so that one bad post
// can't stop a worker.
func (p *Publisher) handle(ctx context.Context, msg queue.Message) {
	now := p.clock.Now()

	// If the post was canceled, already published, or taken by another
	// worker, there's nothing to do.
	post, err := p.store.BeginPublish(ctx, msg.PostID, now)
	if err != nil {
		if errors.Is(err, store.ErrNotClaimable) || errors.Is(err, store.ErrNotFound) {
			p.log.Debug("dropping non-claimable message", "post_id", msg.PostID, "reason", err)
			return
		}
		p.log.Error("begin publish failed", "post_id", msg.PostID, "err", err)
		return
	}

	prov, err := p.registry.Get(post.Target)
	if err != nil {
		_, _ = p.store.FailPublish(ctx, post.ID, "unknown target: "+post.Target, p.clock.Now())
		return
	}

	externalID, pubErr := prov.Publish(ctx, *post)
	if pubErr == nil {
		if _, err := p.store.FinishPublish(ctx, post.ID, externalID, p.clock.Now()); err != nil {
			p.log.Error("finish publish failed", "post_id", post.ID, "err", err)
		}
		p.log.Info("published post", "post_id", post.ID, "target", post.Target, "external_id", externalID)
		return
	}

	p.retryOrFail(ctx, post, pubErr)
}

func (p *Publisher) retryOrFail(ctx context.Context, post *model.Post, pubErr error) {
	msg := pubErr.Error()
	if errors.Is(pubErr, provider.ErrAuth) || post.Attempts >= p.maxAttempts {
		if _, err := p.store.FailPublish(ctx, post.ID, msg, p.clock.Now()); err != nil {
			p.log.Error("fail publish failed", "post_id", post.ID, "err", err)
		}
		p.log.Warn("post permanently failed", "post_id", post.ID, "attempts", post.Attempts, "err", msg)
		return
	}

	if _, err := p.store.RetryPublish(ctx, post.ID, msg, p.clock.Now()); err != nil {
		p.log.Error("retry publish failed", "post_id", post.ID, "err", err)
		return
	}
	p.log.Warn("post publish failed, will retry", "post_id", post.ID, "attempt", post.Attempts, "err", msg)

	delay := p.backoff(post.Attempts)
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}
	}
	if err := p.queue.Enqueue(ctx, queue.Message{PostID: post.ID}); err != nil && ctx.Err() == nil {
		p.log.Error("re-enqueue failed", "post_id", post.ID, "err", err)
	}
}
