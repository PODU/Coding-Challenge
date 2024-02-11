// Package vault implements tokenize and detokenize on top of the token, crypto
// and store packages. Plaintext is encrypted before it reaches the store.
package vault

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"datavault/internal/crypto"
	"datavault/internal/store"
	"datavault/internal/token"
)

// Options control how values are tokenized.
type Options struct {
	// FormatPreserving makes tokens look like the input (an email, a phone
	// number, ...).
	FormatPreserving bool
	// Deterministic gives equal values the same token, which keeps joins
	// working.
	Deterministic bool
}

// DetokResult is the result for one token.
type DetokResult struct {
	Found bool
	Value string
}

// Vault tokenizes and detokenizes values.
type Vault struct {
	store    store.Store
	keyring  *crypto.Keyring
	dedupKey []byte
	now      func() time.Time
}

// New returns a Vault. dedupKey is the HMAC key for deterministic mode and
// must be different from the encryption key.
func New(s store.Store, kr *crypto.Keyring, dedupKey []byte) *Vault {
	return &Vault{store: s, keyring: kr, dedupKey: dedupKey, now: time.Now}
}

// ErrValueTooLarge means a value is over MaxValueSize.
var ErrValueTooLarge = errors.New("vault: field value too large")

// MaxValueSize is the largest field value Tokenize accepts.
const MaxValueSize = 64 * 1024

// Tokenize stores each field value and returns the tokens keyed by field name.
// id is the caller's own reference and is not used here.
func (v *Vault) Tokenize(ctx context.Context, id string, fields map[string]string, opts Options, requestID string) (map[string]string, error) {
	out := make(map[string]string, len(fields))
	for name, value := range fields {
		if len(value) > MaxValueSize {
			return nil, fmt.Errorf("%w: field %q", ErrValueTooLarge, name)
		}
		tok, err := v.tokenizeValue(ctx, value, opts, requestID)
		if err != nil {
			return nil, err
		}
		out[name] = tok
	}
	return out, nil
}

func (v *Vault) tokenizeValue(ctx context.Context, value string, opts Options, requestID string) (string, error) {
	var dedupHMAC string
	if opts.Deterministic {
		dedupHMAC = v.hmacValue(value)
		if existing, ok, err := v.store.GetDedup(ctx, dedupHMAC); err != nil {
			return "", err
		} else if ok {
			return existing, nil
		}
	}

	format := ""
	if opts.FormatPreserving {
		format = token.DetectFormat(value)
	}

	tok, err := v.mintUniqueToken(ctx, format)
	if err != nil {
		return "", err
	}

	version, nonce, ciphertext, err := v.keyring.Encrypt([]byte(value), []byte(tok))
	if err != nil {
		return "", err
	}

	rec := store.Record{
		Token:      tok,
		KeyVersion: version,
		Nonce:      nonce,
		Ciphertext: ciphertext,
		Format:     format,
		CreatedAt:  v.now().UTC(),
		RequestID:  requestID,
	}
	if err := v.store.Put(ctx, rec); err != nil {
		return "", err
	}

	if opts.Deterministic {
		if err := v.store.PutDedup(ctx, dedupHMAC, tok); err != nil {
			return "", err
		}
	}
	return tok, nil
}

// mintUniqueToken returns a token not already in the store. A collision is
// practically impossible for random tokens, but format-preserving ones have
// much less room.
func (v *Vault) mintUniqueToken(ctx context.Context, format string) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		tok, err := token.New(format)
		if err != nil {
			return "", err
		}
		if _, ok, err := v.store.Get(ctx, tok); err != nil {
			return "", err
		} else if !ok {
			return tok, nil
		}
	}
	return "", errors.New("vault: could not mint a unique token")
}

// Detokenize decrypts each token. Unknown tokens come back with Found=false
// instead of failing the whole request.
func (v *Vault) Detokenize(ctx context.Context, tokens []string) (map[string]DetokResult, error) {
	results := make(map[string]DetokResult, len(tokens))
	for _, tok := range tokens {
		if _, done := results[tok]; done {
			continue
		}
		rec, ok, err := v.store.Get(ctx, tok)
		if err != nil {
			return nil, err
		}
		if !ok {
			results[tok] = DetokResult{Found: false}
			continue
		}
		plaintext, err := v.keyring.Decrypt(rec.KeyVersion, rec.Nonce, rec.Ciphertext, []byte(tok))
		if err != nil {
			// The record exists, so this is a wrong key or corruption: an
			// internal error, not "not found".
			return nil, fmt.Errorf("vault: decrypt token: %w", err)
		}
		results[tok] = DetokResult{Found: true, Value: string(plaintext)}
	}
	return results, nil
}

// Ping checks that the store is reachable.
func (v *Vault) Ping(ctx context.Context) error { return v.store.Ping(ctx) }

func (v *Vault) hmacValue(value string) string {
	mac := hmac.New(sha256.New, v.dedupKey)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
