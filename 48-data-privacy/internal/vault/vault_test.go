package vault

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"datavault/internal/crypto"
	"datavault/internal/store"
)

func newTestVault(t testing.TB, s store.Store) *Vault {
	t.Helper()
	master := make([]byte, crypto.KeySize)
	if _, err := rand.Read(master); err != nil {
		t.Fatal(err)
	}
	kr, err := crypto.NewKeyringSingle(master)
	if err != nil {
		t.Fatal(err)
	}
	dedup := make([]byte, 32)
	rand.Read(dedup)
	return New(s, kr, dedup)
}

func TestTokenizeDetokenizeRoundTrip(t *testing.T) {
	ctx := context.Background()
	v := newTestVault(t, store.NewMemStore())

	fields := map[string]string{
		"ssn":     "123-45-6789",
		"email":   "alice@example.com",
		"phone":   "+1-415-555-0100",
		"unicode": "Zoë Müller 名前",
		"empty":   "",
	}
	tokens, err := v.Tokenize(ctx, "customer-42", fields, Options{}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != len(fields) {
		t.Fatalf("expected %d tokens, got %d", len(fields), len(tokens))
	}

	// Tokens must not equal the plaintext.
	for name, tok := range tokens {
		if tok == fields[name] && fields[name] != "" {
			t.Fatalf("token for %q equals plaintext", name)
		}
	}

	var tokenList []string
	for _, tok := range tokens {
		tokenList = append(tokenList, tok)
	}
	results, err := v.Detokenize(ctx, tokenList)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range fields {
		res := results[tokens[name]]
		if !res.Found {
			t.Fatalf("field %q not found on detokenize", name)
		}
		if res.Value != value {
			t.Fatalf("field %q round-trip mismatch: got %q want %q", name, res.Value, value)
		}
	}
}

func TestDetokenizeNotFoundAndDuplicates(t *testing.T) {
	ctx := context.Background()
	v := newTestVault(t, store.NewMemStore())

	tokens, err := v.Tokenize(ctx, "id", map[string]string{"x": "real"}, Options{}, "req")
	if err != nil {
		t.Fatal(err)
	}
	real := tokens["x"]

	results, err := v.Detokenize(ctx, []string{real, "tok_unknown", real})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("duplicate token should collapse: got %d results", len(results))
	}
	if !results[real].Found || results[real].Value != "real" {
		t.Fatalf("known token result wrong: %+v", results[real])
	}
	if results["tok_unknown"].Found {
		t.Fatal("unknown token should be not-found")
	}
}

func TestDeterministicTokenization(t *testing.T) {
	ctx := context.Background()
	v := newTestVault(t, store.NewMemStore())

	t1, _ := v.Tokenize(ctx, "id", map[string]string{"ssn": "123-45-6789"}, Options{Deterministic: true}, "r1")
	t2, _ := v.Tokenize(ctx, "id", map[string]string{"ssn": "123-45-6789"}, Options{Deterministic: true}, "r2")
	if t1["ssn"] != t2["ssn"] {
		t.Fatalf("deterministic mode produced different tokens: %q vs %q", t1["ssn"], t2["ssn"])
	}

	// Default (random) mode produces different tokens.
	r1, _ := v.Tokenize(ctx, "id", map[string]string{"ssn": "123-45-6789"}, Options{}, "r3")
	r2, _ := v.Tokenize(ctx, "id", map[string]string{"ssn": "123-45-6789"}, Options{}, "r4")
	if r1["ssn"] == r2["ssn"] {
		t.Fatal("random mode unexpectedly produced identical tokens")
	}
}

func TestValueTooLarge(t *testing.T) {
	ctx := context.Background()
	v := newTestVault(t, store.NewMemStore())
	big := string(make([]byte, MaxValueSize+1))
	if _, err := v.Tokenize(ctx, "id", map[string]string{"x": big}, Options{}, "r"); err == nil {
		t.Fatal("expected ErrValueTooLarge")
	}
}

// TestEncryptionAtRest tokenizes secrets into a real bbolt file, then scans the
// raw file bytes to assert no plaintext PII is present on disk.
func TestEncryptionAtRest(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "atrest.db")
	bs, err := store.NewBoltStore(path)
	if err != nil {
		t.Fatal(err)
	}
	v := newTestVault(t, bs)

	secrets := []string{"123-45-6789", "alice@secret-domain.com", "+1-415-555-0199"}
	fields := map[string]string{}
	for i, s := range secrets {
		fields[string(rune('a'+i))] = s
	}
	if _, err := v.Tokenize(ctx, "id", fields, Options{}, "req"); err != nil {
		t.Fatal(err)
	}
	if err := bs.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range secrets {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("plaintext %q found in on-disk bbolt file — not encrypted at rest", secret)
		}
	}
}

// TestPersistenceAcrossReopen tokenizes, closes the store, reopens a new vault on
// the same file with the same key, and detokenizes successfully.
func TestPersistenceAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reopen.db")
	master := make([]byte, crypto.KeySize)
	rand.Read(master)
	dedup := make([]byte, 32)
	rand.Read(dedup)
	kr, _ := crypto.NewKeyringSingle(master)

	bs1, err := store.NewBoltStore(path)
	if err != nil {
		t.Fatal(err)
	}
	v1 := New(bs1, kr, dedup)
	tokens, err := v1.Tokenize(ctx, "id", map[string]string{"ssn": "999-88-7777"}, Options{}, "req")
	if err != nil {
		t.Fatal(err)
	}
	tok := tokens["ssn"]
	if err := bs1.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen with the same key material.
	bs2, err := store.NewBoltStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer bs2.Close()
	kr2, _ := crypto.NewKeyringSingle(master)
	v2 := New(bs2, kr2, dedup)

	results, err := v2.Detokenize(ctx, []string{tok})
	if err != nil {
		t.Fatal(err)
	}
	if !results[tok].Found || results[tok].Value != "999-88-7777" {
		t.Fatalf("value did not survive reopen: %+v", results[tok])
	}
}

// FuzzTokenizeRoundTrip asserts detokenize(tokenize(x)) == x for arbitrary
// values up to the size cap.
func FuzzTokenizeRoundTrip(f *testing.F) {
	for _, seed := range []string{"", "a", "123-45-6789", "Zoë", "\x00\x01\x02", strings.Repeat("x", 1000)} {
		f.Add(seed)
	}
	v := newTestVault(f, store.NewMemStore())
	ctx := context.Background()
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > MaxValueSize {
			t.Skip()
		}
		tokens, err := v.Tokenize(ctx, "id", map[string]string{"f": value}, Options{}, "req")
		if err != nil {
			t.Fatal(err)
		}
		results, err := v.Detokenize(ctx, []string{tokens["f"]})
		if err != nil {
			t.Fatal(err)
		}
		if r := results[tokens["f"]]; !r.Found || r.Value != value {
			t.Fatalf("round-trip mismatch for %q: %+v", value, r)
		}
	})
}
