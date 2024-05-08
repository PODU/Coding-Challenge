package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pastebin/internal/paste"
	"pastebin/internal/store"
)

// factory builds a fresh store and a cleanup func.
type factory struct {
	name string
	make func(t *testing.T) store.Store
}

func factories() []factory {
	return []factory{
		{
			name: "mem",
			make: func(t *testing.T) store.Store { return store.NewMemStore() },
		},
		{
			name: "sqlite",
			make: func(t *testing.T) store.Store {
				// Hermetic: a fresh on-disk temp DB per test (auto-removed).
				dsn := t.TempDir() + "/test.db"
				s, err := store.NewSQLiteStore(dsn)
				if err != nil {
					t.Fatalf("open sqlite: %v", err)
				}
				t.Cleanup(func() { s.Close() })
				return s
			},
		},
	}
}

func samplePaste(id string, expiresAt *time.Time) paste.Paste {
	return paste.Paste{
		ID:        id,
		Content:   "hello world",
		Language:  "go",
		CreatedAt: time.Unix(1_700_000_000, 0).UTC(),
		ExpiresAt: expiresAt,
		DeleteKey: "secretkey",
	}
}

// TestStoreContract runs the same suite against every implementation.
func TestStoreContract(t *testing.T) {
	for _, f := range factories() {
		f := f
		t.Run(f.name, func(t *testing.T) {
			t.Run("CreateGetRoundTrip", func(t *testing.T) {
				s := f.make(t)
				ctx := context.Background()
				want := samplePaste("abc123", nil)
				if err := s.Create(ctx, want); err != nil {
					t.Fatalf("create: %v", err)
				}
				got, err := s.Get(ctx, "abc123")
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				if got.ID != want.ID || got.Content != want.Content || got.Language != want.Language || got.DeleteKey != want.DeleteKey {
					t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
				}
				if !got.CreatedAt.Equal(want.CreatedAt) {
					t.Fatalf("created_at: got %v want %v", got.CreatedAt, want.CreatedAt)
				}
				if got.ExpiresAt != nil {
					t.Fatalf("expected nil expiry, got %v", got.ExpiresAt)
				}
			})

			t.Run("GetUnknownNotFound", func(t *testing.T) {
				s := f.make(t)
				_, err := s.Get(context.Background(), "missing")
				if !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("got %v, want ErrNotFound", err)
				}
			})

			t.Run("DuplicateConflict", func(t *testing.T) {
				s := f.make(t)
				ctx := context.Background()
				p := samplePaste("dup", nil)
				if err := s.Create(ctx, p); err != nil {
					t.Fatalf("first create: %v", err)
				}
				err := s.Create(ctx, p)
				if !errors.Is(err, store.ErrConflict) {
					t.Fatalf("got %v, want ErrConflict", err)
				}
			})

			t.Run("DeleteThenGetNotFound", func(t *testing.T) {
				s := f.make(t)
				ctx := context.Background()
				p := samplePaste("del", nil)
				if err := s.Create(ctx, p); err != nil {
					t.Fatalf("create: %v", err)
				}
				if err := s.Delete(ctx, "del"); err != nil {
					t.Fatalf("delete: %v", err)
				}
				if _, err := s.Get(ctx, "del"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("get after delete: got %v, want ErrNotFound", err)
				}
				if err := s.Delete(ctx, "del"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("delete again: got %v, want ErrNotFound", err)
				}
			})

			t.Run("DeleteExpiredReapsOnlyExpired", func(t *testing.T) {
				s := f.make(t)
				ctx := context.Background()
				now := time.Unix(2_000_000_000, 0).UTC()
				past := now.Add(-time.Hour)
				future := now.Add(time.Hour)

				if err := s.Create(ctx, samplePaste("expired1", &past)); err != nil {
					t.Fatal(err)
				}
				if err := s.Create(ctx, samplePaste("expired2", &now)); err != nil { // exactly now => expired
					t.Fatal(err)
				}
				if err := s.Create(ctx, samplePaste("live", &future)); err != nil {
					t.Fatal(err)
				}
				if err := s.Create(ctx, samplePaste("never", nil)); err != nil {
					t.Fatal(err)
				}

				n, err := s.DeleteExpired(ctx, now)
				if err != nil {
					t.Fatalf("delete expired: %v", err)
				}
				if n != 2 {
					t.Fatalf("reaped %d, want 2", n)
				}
				// Live and never-expiry survive; expired are gone.
				for _, id := range []string{"live", "never"} {
					if _, err := s.Get(ctx, id); err != nil {
						t.Fatalf("%s should survive: %v", id, err)
					}
				}
				for _, id := range []string{"expired1", "expired2"} {
					if _, err := s.Get(ctx, id); !errors.Is(err, store.ErrNotFound) {
						t.Fatalf("%s should be reaped: %v", id, err)
					}
				}
			})

			t.Run("PingOK", func(t *testing.T) {
				s := f.make(t)
				if err := s.Ping(context.Background()); err != nil {
					t.Fatalf("ping: %v", err)
				}
			})
		})
	}
}
