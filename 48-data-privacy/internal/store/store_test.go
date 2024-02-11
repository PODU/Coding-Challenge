package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// factories returns a named set of Store constructors so the shared interface
// tests run against every backend.
func factories(t *testing.T) map[string]func() Store {
	return map[string]func() Store{
		"mem": func() Store { return NewMemStore() },
		"bolt": func() Store {
			path := filepath.Join(t.TempDir(), "test.db")
			s, err := NewBoltStore(path)
			if err != nil {
				t.Fatal(err)
			}
			return s
		},
	}
}

func sampleRecord(token string) Record {
	return Record{
		Token:      token,
		KeyVersion: 1,
		Nonce:      []byte{1, 2, 3},
		Ciphertext: []byte("ciphertext-bytes"),
		Format:     "ssn",
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
		RequestID:  "req-1",
	}
}

func TestStoreInterface(t *testing.T) {
	ctx := context.Background()
	for name, factory := range factories(t) {
		t.Run(name, func(t *testing.T) {
			s := factory()
			defer s.Close()

			// Missing key.
			if _, ok, err := s.Get(ctx, "nope"); err != nil || ok {
				t.Fatalf("get missing: ok=%v err=%v", ok, err)
			}

			// Put / Get round-trip.
			rec := sampleRecord("tok_1")
			if err := s.Put(ctx, rec); err != nil {
				t.Fatal(err)
			}
			got, ok, err := s.Get(ctx, "tok_1")
			if err != nil || !ok {
				t.Fatalf("get put: ok=%v err=%v", ok, err)
			}
			if string(got.Ciphertext) != "ciphertext-bytes" || got.KeyVersion != 1 {
				t.Fatalf("record mismatch: %+v", got)
			}

			// Overwrite.
			rec2 := sampleRecord("tok_1")
			rec2.Ciphertext = []byte("updated")
			if err := s.Put(ctx, rec2); err != nil {
				t.Fatal(err)
			}
			got, _, _ = s.Get(ctx, "tok_1")
			if string(got.Ciphertext) != "updated" {
				t.Fatalf("overwrite failed: %q", got.Ciphertext)
			}

			// Dedup index.
			if _, ok, _ := s.GetDedup(ctx, "h1"); ok {
				t.Fatal("dedup should be empty")
			}
			if err := s.PutDedup(ctx, "h1", "tok_1"); err != nil {
				t.Fatal(err)
			}
			tok, ok, err := s.GetDedup(ctx, "h1")
			if err != nil || !ok || tok != "tok_1" {
				t.Fatalf("dedup get: tok=%q ok=%v err=%v", tok, ok, err)
			}

			// Ping.
			if err := s.Ping(ctx); err != nil {
				t.Fatalf("ping: %v", err)
			}
		})
	}
}

func TestStoreConcurrent(t *testing.T) {
	ctx := context.Background()
	for name, factory := range factories(t) {
		t.Run(name, func(t *testing.T) {
			s := factory()
			defer s.Close()
			var wg sync.WaitGroup
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					tok := fmt.Sprintf("tok_%d", i)
					if err := s.Put(ctx, sampleRecord(tok)); err != nil {
						t.Errorf("put: %v", err)
						return
					}
					if _, ok, err := s.Get(ctx, tok); err != nil || !ok {
						t.Errorf("get: ok=%v err=%v", ok, err)
					}
				}(i)
			}
			wg.Wait()
		})
	}
}

// TestBoltPersistenceAcrossReopen verifies records survive a close+reopen of the
// same bbolt file.
func TestBoltPersistenceAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "persist.db")

	s1, err := NewBoltStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Put(ctx, sampleRecord("tok_persist")); err != nil {
		t.Fatal(err)
	}
	if err := s1.PutDedup(ctx, "hmacX", "tok_persist"); err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := NewBoltStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	got, ok, err := s2.Get(ctx, "tok_persist")
	if err != nil || !ok {
		t.Fatalf("record did not persist across reopen: ok=%v err=%v", ok, err)
	}
	if string(got.Ciphertext) != "ciphertext-bytes" {
		t.Fatalf("persisted record corrupted: %+v", got)
	}
	if tok, ok, _ := s2.GetDedup(ctx, "hmacX"); !ok || tok != "tok_persist" {
		t.Fatalf("dedup index did not persist: tok=%q ok=%v", tok, ok)
	}
}

func TestClosedStoreErrors(t *testing.T) {
	s := NewMemStore()
	_ = s.Close()
	if err := s.Put(context.Background(), sampleRecord("x")); err == nil {
		t.Fatal("expected error on closed store")
	}
}
