package paste_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"pastebin/internal/clock"
	"pastebin/internal/paste"
	"pastebin/internal/store"
)

func newSvc(t *testing.T, clk clock.Clock, cfg paste.Config) (*paste.Service, store.Store) {
	t.Helper()
	st := store.NewMemStore()
	return paste.NewService(st, clk, cfg), st
}

func fixedClock() *clock.Fake {
	return clock.NewFake(time.Date(2026, 6, 26, 14, 0, 0, 0, time.UTC))
}

func TestCreateAndRetrieve(t *testing.T) {
	clk := fixedClock()
	svc, _ := newSvc(t, clk, paste.Config{MaxBytes: 1 << 20})
	ctx := context.Background()

	res, err := svc.Create(ctx, paste.CreateParams{Content: "hello world", Language: "go"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.Paste.ID == "" {
		t.Fatal("expected non-empty id")
	}
	if res.DeleteKey == "" {
		t.Fatal("expected non-empty delete key")
	}
	if res.Paste.Language != "go" {
		t.Fatalf("language: got %q", res.Paste.Language)
	}

	got, err := svc.Get(ctx, res.Paste.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Content != "hello world" {
		t.Fatalf("content: got %q", got.Content)
	}
	// Raw exposure: content is verbatim, no transformation.
	if got.Content != res.Paste.Content {
		t.Fatal("raw content differs from stored content")
	}
}

func TestEmptyContentRejected(t *testing.T) {
	svc, _ := newSvc(t, fixedClock(), paste.Config{MaxBytes: 1 << 20})
	_, err := svc.Create(context.Background(), paste.CreateParams{Content: ""})
	if !errors.Is(err, paste.ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

func TestSizeLimit(t *testing.T) {
	const max = 100
	svc, _ := newSvc(t, fixedClock(), paste.Config{MaxBytes: max})
	ctx := context.Background()

	atLimit := strings.Repeat("a", max)
	if _, err := svc.Create(ctx, paste.CreateParams{Content: atLimit}); err != nil {
		t.Fatalf("at-limit content should be accepted: %v", err)
	}
	over := strings.Repeat("a", max+1)
	if _, err := svc.Create(ctx, paste.CreateParams{Content: over}); !errors.Is(err, paste.ErrTooLarge) {
		t.Fatalf("got %v, want ErrTooLarge", err)
	}
}

func TestExpiryViaFakeClock(t *testing.T) {
	clk := fixedClock()
	svc, _ := newSvc(t, clk, paste.Config{MaxBytes: 1 << 20})
	ctx := context.Background()

	res, err := svc.Create(ctx, paste.CreateParams{Content: "ttl", ExpiresIn: "1h"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := res.Paste.ID

	// +59m: still available.
	clk.Advance(59 * time.Minute)
	if _, err := svc.Get(ctx, id); err != nil {
		t.Fatalf("at +59m want OK, got %v", err)
	}

	// +61m total: expired.
	clk.Advance(2 * time.Minute)
	if _, err := svc.Get(ctx, id); !errors.Is(err, paste.ErrExpired) {
		t.Fatalf("at +61m want ErrExpired, got %v", err)
	}
}

func TestNeverExpires(t *testing.T) {
	clk := fixedClock()
	svc, _ := newSvc(t, clk, paste.Config{MaxBytes: 1 << 20})
	ctx := context.Background()

	res, err := svc.Create(ctx, paste.CreateParams{Content: "forever", ExpiresIn: "never"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	clk.Advance(100 * 365 * 24 * time.Hour)
	if _, err := svc.Get(ctx, res.Paste.ID); err != nil {
		t.Fatalf("nil-TTL paste should never expire, got %v", err)
	}
}

func TestBadExpiresIn(t *testing.T) {
	svc, _ := newSvc(t, fixedClock(), paste.Config{MaxBytes: 1 << 20})
	_, err := svc.Create(context.Background(), paste.CreateParams{Content: "x", ExpiresIn: "banana"})
	if !errors.Is(err, paste.ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

func TestNegativeExpiresIn(t *testing.T) {
	svc, _ := newSvc(t, fixedClock(), paste.Config{MaxBytes: 1 << 20})
	_, err := svc.Create(context.Background(), paste.CreateParams{Content: "x", ExpiresIn: "-1h"})
	if !errors.Is(err, paste.ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

func TestTTLExceedsMax(t *testing.T) {
	svc, _ := newSvc(t, fixedClock(), paste.Config{MaxBytes: 1 << 20, MaxTTL: time.Hour})
	ctx := context.Background()
	if _, err := svc.Create(ctx, paste.CreateParams{Content: "x", ExpiresIn: "30m"}); err != nil {
		t.Fatalf("within max should pass: %v", err)
	}
	if _, err := svc.Create(ctx, paste.CreateParams{Content: "x", ExpiresIn: "2h"}); !errors.Is(err, paste.ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

func TestUnknownLanguageRejected(t *testing.T) {
	svc, _ := newSvc(t, fixedClock(), paste.Config{MaxBytes: 1 << 20})
	_, err := svc.Create(context.Background(), paste.CreateParams{Content: "x", Language: "nonsense"})
	if !errors.Is(err, paste.ErrInvalid) {
		t.Fatalf("got %v, want ErrInvalid", err)
	}
}

func TestDelete(t *testing.T) {
	clk := fixedClock()
	svc, _ := newSvc(t, clk, paste.Config{MaxBytes: 1 << 20})
	ctx := context.Background()

	res, err := svc.Create(ctx, paste.CreateParams{Content: "to delete"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := res.Paste.ID

	// Wrong key -> forbidden.
	if err := svc.Delete(ctx, id, "wrong"); !errors.Is(err, paste.ErrForbidden) {
		t.Fatalf("wrong key: got %v, want ErrForbidden", err)
	}
	// Empty key -> forbidden.
	if err := svc.Delete(ctx, id, ""); !errors.Is(err, paste.ErrForbidden) {
		t.Fatalf("empty key: got %v, want ErrForbidden", err)
	}
	// Right key -> deleted.
	if err := svc.Delete(ctx, id, res.DeleteKey); err != nil {
		t.Fatalf("right key: %v", err)
	}
	// Now gone.
	if _, err := svc.Get(ctx, id); !errors.Is(err, paste.ErrNotFound) {
		t.Fatalf("after delete: got %v, want ErrNotFound", err)
	}
	// Delete unknown -> not found.
	if err := svc.Delete(ctx, "nope", "k"); !errors.Is(err, paste.ErrNotFound) {
		t.Fatalf("delete unknown: got %v, want ErrNotFound", err)
	}
}

func TestGetUnknownNotFound(t *testing.T) {
	svc, _ := newSvc(t, fixedClock(), paste.Config{MaxBytes: 1 << 20})
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, paste.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

// conflictOnceStore returns ErrConflict on the first Create, then delegates.
type conflictOnceStore struct {
	inner  paste.Store
	failed bool
}

func (s *conflictOnceStore) Create(ctx context.Context, p paste.Paste) error {
	if !s.failed {
		s.failed = true
		return store.ErrConflict
	}
	return s.inner.Create(ctx, p)
}
func (s *conflictOnceStore) Get(ctx context.Context, id string) (paste.Paste, error) {
	return s.inner.Get(ctx, id)
}
func (s *conflictOnceStore) Delete(ctx context.Context, id string) error {
	return s.inner.Delete(ctx, id)
}

func TestIDCollisionRetry(t *testing.T) {
	st := &conflictOnceStore{inner: store.NewMemStore()}
	svc := paste.NewService(st, fixedClock(), paste.Config{MaxBytes: 1 << 20})
	res, err := svc.Create(context.Background(), paste.CreateParams{Content: "retry me"})
	if err != nil {
		t.Fatalf("expected success after one conflict, got %v", err)
	}
	if !st.failed {
		t.Fatal("expected the stub to have returned a conflict at least once")
	}
	if res.Paste.ID == "" {
		t.Fatal("expected a valid paste after retry")
	}
}
