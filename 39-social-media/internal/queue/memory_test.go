package queue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryFIFO(t *testing.T) {
	q := NewMemory(8)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := q.Enqueue(ctx, Message{PostID: string(rune('a' + i))}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		m, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("dequeue: %v", err)
		}
		want := string(rune('a' + i))
		if m.PostID != want {
			t.Fatalf("got %q want %q (not FIFO)", m.PostID, want)
		}
	}
}

func TestMemoryBackpressure(t *testing.T) {
	q := NewMemory(1)
	ctx := context.Background()
	if err := q.Enqueue(ctx, Message{PostID: "1"}); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}

	// Second enqueue must block until a slot frees up.
	cctx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	err := q.Enqueue(cctx, Message{PostID: "2"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected blocking until deadline, got %v", err)
	}

	// Drain and confirm a subsequent enqueue now succeeds.
	if _, err := q.Dequeue(ctx); err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if err := q.Enqueue(ctx, Message{PostID: "3"}); err != nil {
		t.Fatalf("enqueue after drain: %v", err)
	}
}

func TestMemoryCloseDrains(t *testing.T) {
	q := NewMemory(4)
	ctx := context.Background()
	_ = q.Enqueue(ctx, Message{PostID: "x"})
	q.Close()

	// Buffered message is still delivered after close.
	m, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("expected buffered message, got %v", err)
	}
	if m.PostID != "x" {
		t.Fatalf("got %q", m.PostID)
	}
	// Now empty + closed → ErrClosed.
	if _, err := q.Dequeue(ctx); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
	// Enqueue after close → ErrClosed.
	if err := q.Enqueue(ctx, Message{PostID: "y"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed on enqueue, got %v", err)
	}
}

func TestMemoryCloseIdempotent(t *testing.T) {
	q := NewMemory(1)
	q.Close()
	q.Close() // must not panic
}
