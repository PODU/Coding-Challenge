// Package broker defines the messaging boundary (the Kafka seam). Producers and
// consumers are interfaces so the pipeline can be tested against an in-process
// implementation with no real broker.
package broker

import (
	"context"

	"github.com/codingchallenges/topstories/internal/model"
)

// Producer publishes stories to the message bus.
type Producer interface {
	Publish(ctx context.Context, s model.Story) error
	Close() error
}

// Consumer delivers stories from the message bus.
type Consumer interface {
	// Consume invokes handle for each story until ctx is cancelled. A handle
	// error is logged by the implementation and does not stop consumption.
	Consume(ctx context.Context, handle func(model.Story) error) error
	Close() error
}
