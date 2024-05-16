// Package hash has SHA-256 helpers. Changes are decided by content hash; size
// and mtime only let us skip re-hashing.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// Reader streams r through SHA-256, returning the hex digest and byte count.
func Reader(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// File returns the hex SHA-256 of the file at path.
func File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum, _, err := Reader(f)
	return sum, err
}

// Bytes returns the hex SHA-256 of b.
func Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
