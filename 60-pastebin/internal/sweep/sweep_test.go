package sweep_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pastebin/internal/clock"
	"pastebin/internal/paste"
	"pastebin/internal/store"
	"pastebin/internal/sweep"
)

func TestSweeperRunOnce(t *testing.T) {
	st := store.NewMemStore()
	now := time.Date(2026, 6, 26, 14, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	ctx := context.Background()

	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	mk := func(id string, exp *time.Time) paste.Paste {
		return paste.Paste{ID: id, Content: "c", CreatedAt: now, ExpiresAt: exp, DeleteKey: "k"}
	}
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(st.Create(ctx, mk("expired1", &past)))
	must(st.Create(ctx, mk("expired2", &past)))
	must(st.Create(ctx, mk("live", &future)))
	must(st.Create(ctx, mk("never", nil)))

	s := sweep.New(st, clk, time.Minute, nil)
	n, err := s.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if n != 2 {
		t.Fatalf("reaped %d, want 2", n)
	}

	for _, id := range []string{"live", "never"} {
		if _, err := st.Get(ctx, id); err != nil {
			t.Fatalf("%s should survive: %v", id, err)
		}
	}
	for _, id := range []string{"expired1", "expired2"} {
		if _, err := st.Get(ctx, id); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("%s should be reaped: %v", id, err)
		}
	}

	// Advance the clock so the previously-live paste expires, then sweep again.
	clk.Set(future.Add(time.Minute))
	n, err = s.RunOnce(ctx)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if n != 1 {
		t.Fatalf("second sweep reaped %d, want 1", n)
	}
}
