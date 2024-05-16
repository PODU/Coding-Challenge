package server

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"dropbox/internal/hash"
)

func TestPutGetVersioning(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	body := []byte("hello")
	fi, err := s.Put("alice", "a/b.txt", 0, hash.Bytes(body), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Version != 1 || fi.Hash != hash.Bytes(body) || fi.Size != 5 {
		t.Fatalf("unexpected info %+v", fi)
	}
	// Stale If-Match must conflict.
	if _, err := s.Put("alice", "a/b.txt", 0, hash.Bytes([]byte("x")), bytes.NewReader([]byte("x"))); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("want version conflict, got %v", err)
	}
	// Correct If-Match bumps to 2.
	body2 := []byte("hello2")
	fi2, err := s.Put("alice", "a/b.txt", 1, hash.Bytes(body2), bytes.NewReader(body2))
	if err != nil {
		t.Fatal(err)
	}
	if fi2.Version != 2 {
		t.Fatalf("version = %d, want 2", fi2.Version)
	}
	f, got, err := s.Get("alice", "a/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "hello2" || got.Version != 2 {
		t.Fatalf("get mismatch: %q v%d", data, got.Version)
	}
}

func TestPutHashMismatch(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	_, err := s.Put("alice", "a.txt", -1, "deadbeef", bytes.NewReader([]byte("real")))
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("want hash mismatch, got %v", err)
	}
	// Store must be untouched.
	if _, _, err := s.Get("alice", "a.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found after rejected put, got %v", err)
	}
}

func TestPathSafety(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	for _, bad := range []string{"../escape", "/abs/path", "a/../../b", "..", "", "a/./b"} {
		if _, err := s.Put("alice", bad, -1, "", strings.NewReader("x")); !errors.Is(err, ErrBadPath) {
			t.Errorf("Put(%q) err = %v, want ErrBadPath", bad, err)
		}
	}
}

func TestDeleteIdempotentAndVersioned(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	// Deleting an absent path is a no-op.
	if err := s.Delete("alice", "ghost", -1); err != nil {
		t.Fatalf("delete absent: %v", err)
	}
	body := []byte("data")
	if _, err := s.Put("alice", "f", -1, hash.Bytes(body), bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("alice", "f", 99); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("want conflict on stale delete, got %v", err)
	}
	if err := s.Delete("alice", "f", 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, err := s.Get("alice", "f"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found after delete, got %v", err)
	}
}

// TestConcurrentPutSamePath is the FR-8 multi-client safety check: many
// goroutines race to create the same path with If-Match 0; exactly one wins and
// the index stays consistent. Run with -race.
func TestConcurrentPutSamePath(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	const n = 32
	var wins int64
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := []byte{byte(i)}
			_, err := s.Put("alice", "race.bin", 0, hash.Bytes(body), bytes.NewReader(body))
			switch {
			case err == nil:
				atomic.AddInt64(&wins, 1)
			case errors.Is(err, ErrVersionConflict):
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("exactly one PUT should win, got %d", wins)
	}
	files, err := s.List("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Version != 1 {
		t.Fatalf("index inconsistent: %+v", files)
	}
	// The committed blob must match the recorded hash.
	f, fi, err := s.Get("alice", "race.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if hash.Bytes(data) != fi.Hash {
		t.Fatalf("stored blob hash != index hash")
	}
}

func TestListSortedAndScoped(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir())
	put := func(user, p, c string) {
		if _, err := s.Put(user, p, -1, hash.Bytes([]byte(c)), strings.NewReader(c)); err != nil {
			t.Fatal(err)
		}
	}
	put("alice", "z", "1")
	put("alice", "a", "2")
	put("bob", "secret", "3")

	al, _ := s.List("alice")
	if len(al) != 2 || al[0].Path != "a" || al[1].Path != "z" {
		t.Fatalf("alice list wrong: %+v", al)
	}
	bob, _ := s.List("bob")
	if len(bob) != 1 || bob[0].Path != "secret" {
		t.Fatalf("bob list wrong: %+v", bob)
	}
}
