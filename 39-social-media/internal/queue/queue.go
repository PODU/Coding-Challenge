// Package queue carries post IDs from the API and scheduler to the publisher
// workers. The interface is small enough that a real broker (RabbitMQ, SQS,
// Redis Streams) could stand in for the in-memory one.
package queue

import (
	"context"
	"errors"
)

var ErrClosed = errors.New("queue: closed")

// Message carries only a post ID. The store has the real state, so consumers
// always load the post before acting on it.
type Message struct {
	PostID string
}

// Queue is a FIFO queue. Enqueue blocks while the queue is full. Dequeue
// blocks while it is empty, and returns ErrClosed once it has been closed and
// emptied. Close wakes up anyone blocked in either.
type Queue interface {
	Enqueue(ctx context.Context, m Message) error
	Dequeue(ctx context.Context) (m Message, err error)
	Close()
}
