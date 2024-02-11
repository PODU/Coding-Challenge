// Package crypto encrypts vault records with AES-256-GCM. Keys are versioned
// so they can be rotated: new records use the current key and older keys are
// kept for decryption only.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// KeySize is the length of an AES-256 key in bytes.
const KeySize = 32

// NonceSize is the standard 96-bit GCM nonce length.
const NonceSize = 12

var (
	ErrKeySize           = fmt.Errorf("crypto: key must be %d bytes", KeySize)
	ErrUnknownKeyVersion = errors.New("crypto: unknown key version")
	// ErrDecrypt covers every decryption failure. It is deliberately vague so
	// nothing about the cause leaks to callers.
	ErrDecrypt = errors.New("crypto: decryption failed")
)

// Keyring holds AES-256 keys by version. New data is encrypted with the
// current version.
type Keyring struct {
	current byte
	aeads   map[byte]cipher.AEAD
}

// NewKeyring builds a keyring. currentVersion must be one of keys.
func NewKeyring(currentVersion byte, keys map[byte][]byte) (*Keyring, error) {
	if len(keys) == 0 {
		return nil, errors.New("crypto: keyring requires at least one key")
	}
	if _, ok := keys[currentVersion]; !ok {
		return nil, errors.New("crypto: current key version not present in keyring")
	}
	aeads := make(map[byte]cipher.AEAD, len(keys))
	for version, key := range keys {
		if len(key) != KeySize {
			return nil, ErrKeySize
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("crypto: new cipher: %w", err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("crypto: new gcm: %w", err)
		}
		aeads[version] = aead
	}
	return &Keyring{current: currentVersion, aeads: aeads}, nil
}

// NewKeyringSingle returns a keyring holding just key, as version 1.
func NewKeyringSingle(key []byte) (*Keyring, error) {
	return NewKeyring(1, map[byte][]byte{1: key})
}

// CurrentVersion returns the version used for new encryptions.
func (k *Keyring) CurrentVersion() byte { return k.current }

// Encrypt encrypts plaintext with the current key and a random nonce. aad is
// the token; authenticating it means a ciphertext can't be moved to a
// different token.
func (k *Keyring) Encrypt(plaintext, aad []byte) (version byte, nonce, ciphertext []byte, err error) {
	aead := k.aeads[k.current]
	nonce = make([]byte, NonceSize)
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return 0, nil, nil, fmt.Errorf("crypto: read nonce: %w", err)
	}
	ciphertext = aead.Seal(nil, nonce, plaintext, aad)
	return k.current, nonce, ciphertext, nil
}

// Decrypt reverses Encrypt. Any tampering or wrong key gives ErrDecrypt.
func (k *Keyring) Decrypt(version byte, nonce, ciphertext, aad []byte) ([]byte, error) {
	aead, ok := k.aeads[version]
	if !ok {
		return nil, ErrUnknownKeyVersion
	}
	if len(nonce) != NonceSize {
		return nil, ErrDecrypt
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}
