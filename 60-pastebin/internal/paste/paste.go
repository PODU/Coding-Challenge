// Package paste contains the domain model and business logic for pastes.
package paste

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"
)

// Errors returned by Service; the HTTP layer maps them to status codes.
var (
	// ErrInvalid covers bad input such as empty content or a bad TTL.
	ErrInvalid  = errors.New("invalid paste")
	ErrTooLarge = errors.New("paste too large")
	ErrNotFound = errors.New("paste not found")
	ErrExpired  = errors.New("paste expired")
	// ErrForbidden means a missing or wrong delete key.
	ErrForbidden = errors.New("forbidden")
	// ErrConflict is an ID collision on insert. The store package reuses this
	// value, since importing store from here would be an import cycle.
	ErrConflict = errors.New("paste id conflict")
)

func isNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

func isConflict(err error) bool { return errors.Is(err, ErrConflict) }

// Paste is a stored paste.
type Paste struct {
	ID        string     // short, URL-safe identifier; primary key
	Content   string     // raw paste body
	Language  string     // "" or a highlight token such as "go" / "python"
	CreatedAt time.Time  // creation timestamp (UTC)
	ExpiresAt *time.Time // nil = never expires
	DeleteKey string     // secret returned on create; required to delete via API
}

// IsExpired reports whether the paste has expired as of now.
func (p Paste) IsExpired(now time.Time) bool {
	return p.ExpiresAt != nil && !now.Before(*p.ExpiresAt)
}

const (
	deleteKeyBytes = 16 // -> 32 hex chars
	idLen          = 11 // base62 chars needed to encode a 64-bit value
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// newID returns a random idLen-character base62 ID from 64 random bits.
func newID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base62Encode(binary.BigEndian.Uint64(b[:])), nil
}

// base62Encode encodes v in base 62, zero-padded to idLen characters.
func base62Encode(v uint64) string {
	out := make([]byte, idLen)
	for i := idLen - 1; i >= 0; i-- {
		out[i] = base62[v%62]
		v /= 62
	}
	return string(out)
}

func newDeleteKey() (string, error) {
	b := make([]byte, deleteKeyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
