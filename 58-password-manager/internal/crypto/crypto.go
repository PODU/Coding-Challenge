// Package crypto derives a key from the master password with Argon2id and
// encrypts with AES-256-GCM. Callers never handle nonces; Seal makes a fresh
// random one each time.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	KeyLen   = 32 // AES-256
	SaltLen  = 16
	NonceLen = 12 // standard GCM nonce
)

// ErrAuth means decryption failed. GCM can't tell a wrong password from a
// tampered vault, so the message covers both.
var ErrAuth = errors.New("authentication failed: wrong master password, or the vault is corrupt or has been tampered with")

// KDFParams are stored in the vault header, unencrypted but authenticated as
// AAD, so the cost can be raised later without breaking existing vaults.
type KDFParams struct {
	Algorithm string `json:"kdf"`     // "argon2id"
	Time      uint32 `json:"time"`    // iterations
	Memory    uint32 `json:"memory"`  // KiB
	Threads   uint8  `json:"threads"` // lanes
	Salt      []byte `json:"salt"`    // base64 in JSON
}

// DefaultParams returns Argon2id settings suited to interactive use, with a
// new random salt.
func DefaultParams() (KDFParams, error) {
	salt := make([]byte, SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return KDFParams{}, fmt.Errorf("generating salt: %w", err)
	}
	return KDFParams{
		Algorithm: "argon2id",
		Time:      3,
		Memory:    64 * 1024, // 64 MiB
		Threads:   4,
		Salt:      salt,
	}, nil
}

func (p KDFParams) Validate() error {
	if p.Algorithm != "argon2id" {
		return fmt.Errorf("unsupported KDF %q", p.Algorithm)
	}
	if p.Time == 0 || p.Memory == 0 || p.Threads == 0 {
		return errors.New("invalid KDF parameters: time, memory and threads must be > 0")
	}
	if len(p.Salt) != SaltLen {
		return fmt.Errorf("invalid salt length %d, want %d", len(p.Salt), SaltLen)
	}
	return nil
}

// DeriveKey turns the master password into a KeyLen-byte key.
func DeriveKey(password []byte, p KDFParams) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return argon2.IDKey(password, p.Salt, p.Time, p.Memory, p.Threads, KeyLen), nil
}

// Seal encrypts plaintext and authenticates it together with aad, using a new
// random nonce.
func Seal(key, plaintext, aad []byte) (nonce, ciphertext []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, NonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, aad)
	return nonce, ciphertext, nil
}

// Open reverses Seal. A wrong key or any altered input gives ErrAuth.
func Open(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce length %d, want %d", len(nonce), gcm.NonceSize())
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrAuth
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeyLen {
		return nil, fmt.Errorf("invalid key length %d, want %d", len(key), KeyLen)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	return gcm, nil
}

func MarshalParams(p KDFParams) ([]byte, error) {
	return json.Marshal(p)
}

func UnmarshalParams(b []byte) (KDFParams, error) {
	var p KDFParams
	if err := json.Unmarshal(b, &p); err != nil {
		return KDFParams{}, fmt.Errorf("parsing KDF header: %w", err)
	}
	return p, nil
}

// Zero overwrites a secret. The GC may already have copied it elsewhere, so
// this only narrows the window.
func Zero(b []byte) {
	clear(b)
}
