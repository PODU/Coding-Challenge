package queue

import (
	"context"
	"sync"
)

// Memory is a Queue backed by a buffered channel.
type Memory struct {
	ch     chan Message
	closed chan struct{}
	once   sync.Once
}

// NewMemory returns a queue holding up to capacity messages (at least 1).
func NewMemory(capacity int) *Memory {
	if capacity <= 0 {
		capacity = 1
	}
	return &Memory{
		ch:     make(chan Message, capacity),
		closed: make(chan struct{}),
	}
}

func (q *Memory) Enqueue(ctx context.Context, m Message) error {
	// select picks at random among ready cases, so check closed on its own
	// first or a send could still win after Close.
	select {
	case <-q.closed:
		return ErrClosed
	default:
	}

	select {
	case q.ch <- m:
		return nil
	case <-q.closed:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Memory) Dequeue(ctx context.Context) (Message, error) {
	select {
	case m := <-q.ch:
		return m, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-q.closed:
		// Hand out what's left before reporting closed.
		select {
		case m := <-q.ch:
			return m, nil
		default:
			return Message{}, ErrClosed
		}
	}
}

func (q *Memory) Len() int { return len(q.ch) }

// Close is safe to call more than once.
func (q *Memory) Close() {
	q.once.Do(func() { close(q.closed) })
}
