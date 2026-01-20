// Package memory is a channel-backed implementation of the broker interfaces,
// used in tests and for running without Kafka.
package memory

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/codingchallenges/topstories/internal/broker"
	"github.com/codingchallenges/topstories/internal/model"
)

// ErrClosed is returned when publishing to a closed bus.
var ErrClosed = errors.New("memory broker: closed")

// Bus is a single-topic in-memory message bus. It implements both
// broker.Producer and broker.Consumer.
type Bus struct {
	mu     sync.Mutex
	ch     chan []byte
	closed bool
}

var (
	_ broker.Producer = (*Bus)(nil)
	_ broker.Consumer = (*Bus)(nil)
)

// New creates a Bus with the given channel buffer size.
func New(buffer int) *Bus {
	if buffer < 0 {
		buffer = 0
	}
	return &Bus{ch: make(chan []byte, buffer)}
}

// Publish serializes the story and enqueues it. It honors ctx cancellation.
func (b *Bus) Publish(ctx context.Context, s model.Story) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return ErrClosed
	}
	ch := b.ch
	b.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch <- data:
		return nil
	}
}

// Consume reads stories until ctx is cancelled or the bus is closed. Messages
// that fail to decode are skipped.
func (b *Bus) Consume(ctx context.Context, handle func(model.Story) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case data, ok := <-b.ch:
			if !ok {
				return nil
			}
			var s model.Story
			if err := json.Unmarshal(data, &s); err == nil {
				_ = handle(s)
			}
		}
	}
}

// Close shuts the bus down. Safe to call more than once.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	close(b.ch)
	return nil
}
