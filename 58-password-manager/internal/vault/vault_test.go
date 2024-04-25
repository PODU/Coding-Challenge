package vault

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ccpm/internal/crypto"
)

// testKey builds a vault with cheap KDF params and the matching derived key.
func testKey(t *testing.T) (*Vault, []byte) {
	t.Helper()
	v, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	v.params.Time = 1
	v.params.Memory = 8 * 1024
	key, err := crypto.DeriveKey([]byte("master-pw"), v.params)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	return v, key
}

func TestCRUD(t *testing.T) {
	v, _ := testKey(t)

	if err := v.Add(Record{Name: "github", Username: "alice", Password: "p1"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := v.Add(Record{Name: "github", Username: "x"}); !errors.Is(err, ErrExists) {
		t.Fatalf("expected ErrExists on duplicate, got %v", err)
	}
	if err := v.Add(Record{Name: ""}); err == nil {
		t.Fatal("expected error for empty name")
	}

	r, err := v.Get("github")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if r.Username != "alice" || r.Password != "p1" {
		t.Fatalf("unexpected record %+v", r)
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		t.Fatal("timestamps not set")
	}

	if _, err := v.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	_ = v.Add(Record{Name: "aws", Username: "bob"})
	names := v.List()
	if len(names) != 2 || names[0] != "aws" || names[1] != "github" {
		t.Fatalf("List = %v, want sorted [aws github]", names)
	}

	if err := v.Update("github", func(r *Record) { r.Password = "p2" }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	r, _ = v.Get("github")
	if r.Password != "p2" {
		t.Fatalf("update did not apply: %+v", r)
	}
	if err := v.Update("missing", func(*Record) {}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on update, got %v", err)
	}

	if err := v.Delete("github"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := v.Get("github"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected record gone, got %v", err)
	}
	if err := v.Delete("github"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	v, key := testKey(t)
	_ = v.Add(Record{Name: "a", Username: "u", Password: "secret", Notes: "n"})

	data, err := v.Encode(key)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(data, key)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	r, err := got.Get("a")
	if err != nil || r.Password != "secret" || r.Notes != "n" {
		t.Fatalf("decoded record mismatch: %+v err=%v", r, err)
	}

	// Header survives the round trip.
	p, err := ParseHeader(data)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}
	if p.Algorithm != "argon2id" || len(p.Salt) != crypto.SaltLen {
		t.Fatalf("header params lost: %+v", p)
	}
}

func TestDecodeWrongKey(t *testing.T) {
	v, key := testKey(t)
	_ = v.Add(Record{Name: "a", Password: "s"})
	data, _ := v.Encode(key)

	badKey, _ := crypto.DeriveKey([]byte("wrong"), v.params)
	if _, err := Decode(data, badKey); !errors.Is(err, crypto.ErrAuth) {
		t.Fatalf("expected ErrAuth for wrong key, got %v", err)
	}
}

func TestTamperByteFlip(t *testing.T) {
	v, key := testKey(t)
	_ = v.Add(Record{Name: "a", Password: "s"})
	data, _ := v.Encode(key)

	// Flip a byte in the ciphertext tail.
	bad := append([]byte(nil), data...)
	bad[len(bad)-1] ^= 0x01
	if _, err := Decode(bad, key); !errors.Is(err, crypto.ErrAuth) {
		t.Fatalf("expected ErrAuth after tamper, got %v", err)
	}

	// Flip a byte in the header (authenticated as AAD).
	bad2 := append([]byte(nil), data...)
	bad2[8] ^= 0x01 // inside the JSON header region
	if _, err := Decode(bad2, key); err == nil {
		t.Fatal("expected error after header tamper")
	}
}

func TestDecodeCorrupt(t *testing.T) {
	v, key := testKey(t)
	data, _ := v.Encode(key)

	cases := map[string][]byte{
		"too short":   {0x01, 0x02},
		"bad magic":   append([]byte("XXXX"), data[4:]...),
		"truncated":   data[:10],
		"empty bytes": {},
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(bad, key); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("expected ErrCorrupt, got %v", err)
			}
		})
	}

	// Unsupported version.
	badVer := append([]byte(nil), data...)
	badVer[4] = 0xFF
	if _, err := Decode(badVer, key); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("expected ErrCorrupt for bad version, got %v", err)
	}
}

func TestSaveLoadPersistence(t *testing.T) {
	v, key := testKey(t)
	_ = v.Add(Record{Name: "github", Username: "alice", Password: "hunter2"})
	_ = v.Add(Record{Name: "aws", Username: "bob", Password: "s3cr3t"})

	path := filepath.Join(t.TempDir(), "vault.ccpm")
	if err := v.Save(path, key); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File permissions 0600 (skipped on Windows where mode bits differ).
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("vault file is empty")
	}

	// Reopen in a fresh Vault (simulating a new process).
	reopened, err := Load(path, key)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reopened.List()) != 2 {
		t.Fatalf("expected 2 records after reopen, got %d", len(reopened.List()))
	}
	r, err := reopened.Get("github")
	if err != nil || r.Password != "hunter2" {
		t.Fatalf("persisted record mismatch: %+v err=%v", r, err)
	}
}

func TestNoPlaintextAtRest(t *testing.T) {
	v, key := testKey(t)
	_ = v.Add(Record{Name: "site", Username: "user", Password: "SuperSecretValue123"})
	data, _ := v.Encode(key)
	if containsSub(data, []byte("SuperSecretValue123")) {
		t.Fatal("plaintext password found in encoded vault")
	}
	if containsSub(data, []byte("user")) {
		t.Fatal("plaintext username found in encoded vault")
	}
}

func containsSub(haystack, needle []byte) bool {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}
