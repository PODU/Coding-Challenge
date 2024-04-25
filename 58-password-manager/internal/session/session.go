// Package session caches the derived key on disk for a while so the master
// password isn't needed for every command.
//
// The file holds the key itself, so it's written 0600 in the temp dir. That's
// a deliberate convenience-over-security trade; a zero timeout turns caching
// off.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrExpired means the session file exists but has expired.
var ErrExpired = errors.New("session expired")

// ErrNone means there is no session file.
var ErrNone = errors.New("no active session")

type record struct {
	Key       []byte    `json:"key"`
	ExpiresAt time.Time `json:"expires_at"`
}

// path names the session file after a hash of the vault's absolute path, so
// each vault gets its own file and the name doesn't reveal where the vault is.
func path(vaultPath string) string {
	abs, err := filepath.Abs(vaultPath)
	if err != nil {
		abs = vaultPath
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(os.TempDir(), "ccpm-sess-"+hex.EncodeToString(sum[:8])+".json")
}

// Start caches key for ttl. A ttl of zero or less does nothing.
func Start(vaultPath string, key []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	data, err := json.Marshal(record{Key: key, ExpiresAt: time.Now().Add(ttl)})
	if err != nil {
		return err
	}
	return os.WriteFile(path(vaultPath), data, 0o600)
}

// Load returns the cached key. An expired session is deleted and reported as
// ErrExpired.
func Load(vaultPath string) ([]byte, error) {
	data, err := os.ReadFile(path(vaultPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNone
		}
		return nil, err
	}
	var rec record
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("corrupt session: %w", err)
	}
	if time.Now().After(rec.ExpiresAt) {
		_ = End(vaultPath)
		return nil, ErrExpired
	}
	return rec.Key, nil
}

// End deletes the session, if there is one.
func End(vaultPath string) error {
	if err := os.Remove(path(vaultPath)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
