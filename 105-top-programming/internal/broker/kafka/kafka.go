// Package kafka implements the broker interfaces with segmentio/kafka-go. The
// offline tests use the memory bus instead, so nothing here is covered by them.
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/codingchallenges/topstories/internal/broker"
	"github.com/codingchallenges/topstories/internal/model"
)

// Producer publishes stories to a Kafka topic.
type Producer struct {
	writer *kafkago.Writer
	log    *slog.Logger
}

var _ broker.Producer = (*Producer)(nil)

// NewProducer builds a Kafka-backed producer.
func NewProducer(brokers []string, topic string, log *slog.Logger) *Producer {
	if log == nil {
		log = slog.Default()
	}
	return &Producer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafkago.LeastBytes{},
			RequiredAcks: kafkago.RequireOne,
			WriteTimeout: 10 * time.Second,
			Async:        false,
		},
		log: log,
	}
}

// Publish writes the story as a JSON message keyed by its ID, retrying on
// transient errors with bounded backoff.
func (p *Producer) Publish(ctx context.Context, s model.Story) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	msg := kafkago.Message{
		Key:   []byte(strconv.FormatInt(s.ID, 10)),
		Value: data,
	}
	const maxAttempts = 3
	for attempt := 1; ; attempt++ {
		err = p.writer.WriteMessages(ctx, msg)
		if err == nil {
			return nil
		}
		p.log.Warn("kafka publish failed", "attempt", attempt, "err", err)
		if attempt == maxAttempts {
			return fmt.Errorf("kafka publish after %d attempts: %w", maxAttempts, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
		}
	}
}

// Close flushes and closes the writer.
func (p *Producer) Close() error { return p.writer.Close() }

// Consumer reads stories from a Kafka topic.
type Consumer struct {
	reader *kafkago.Reader
	log    *slog.Logger
}

var _ broker.Consumer = (*Consumer)(nil)

// NewConsumer builds a Kafka-backed consumer in the given group.
func NewConsumer(brokers []string, topic, group string, log *slog.Logger) *Consumer {
	if log == nil {
		log = slog.Default()
	}
	return &Consumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  group,
			MinBytes: 1,
			MaxBytes: 10e6,
		}),
		log: log,
	}
}

// Consume reads until ctx is cancelled. Malformed messages are logged and
// skipped; handle errors are logged but do not stop the loop.
func (c *Consumer) Consume(ctx context.Context, handle func(model.Story) error) error {
	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ctx.Err()
			}
			return err
		}
		var s model.Story
		if err := json.Unmarshal(m.Value, &s); err != nil {
			c.log.Warn("kafka malformed message skipped", "err", err)
			continue
		}
		if err := handle(s); err != nil {
			c.log.Warn("consumer handle error", "id", s.ID, "err", err)
		}
	}
}

// Close closes the reader.
func (c *Consumer) Close() error { return c.reader.Close() }
