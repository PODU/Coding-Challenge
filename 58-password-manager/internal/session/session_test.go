package session

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestStartLoadEnd(t *testing.T) {
	vp := filepath.Join(t.TempDir(), "v.ccpm")
	key := bytes.Repeat([]byte{0xAB}, 32)
	defer End(vp)

	if err := Start(vp, key, time.Minute); err != nil {
		t.Fatalf("Start: %v", err)
	}
	got, err := Load(vp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, key) {
		t.Fatal("loaded key does not match stored key")
	}

	if err := End(vp); err != nil {
		t.Fatalf("End: %v", err)
	}
	if _, err := Load(vp); !errors.Is(err, ErrNone) {
		t.Fatalf("expected ErrNone after End, got %v", err)
	}
}

func TestExpiry(t *testing.T) {
	vp := filepath.Join(t.TempDir(), "v.ccpm")
	key := bytes.Repeat([]byte{0x01}, 32)
	defer End(vp)

	if err := Start(vp, key, -time.Second); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Negative TTL means caching disabled -> no file written.
	if _, err := Load(vp); !errors.Is(err, ErrNone) {
		t.Fatalf("expected ErrNone for disabled caching, got %v", err)
	}

	// Now write a session that expires almost immediately.
	if err := Start(vp, key, time.Millisecond); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := Load(vp); !errors.Is(err, ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
	// Expired session should have been deleted.
	if _, err := Load(vp); !errors.Is(err, ErrNone) {
		t.Fatalf("expected ErrNone after expiry cleanup, got %v", err)
	}
}

func TestNoSession(t *testing.T) {
	vp := filepath.Join(t.TempDir(), "absent.ccpm")
	if _, err := Load(vp); !errors.Is(err, ErrNone) {
		t.Fatalf("expected ErrNone, got %v", err)
	}
}
