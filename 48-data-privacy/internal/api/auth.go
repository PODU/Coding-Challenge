package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	ScopeTokenize   = "tokenize"
	ScopeDetokenize = "detokenize"
)

// Principal is an authenticated caller and the scopes it was granted.
type Principal struct {
	Name   string
	Scopes map[string]bool
}

// Can reports whether the principal holds scope.
func (p *Principal) Can(scope string) bool { return p != nil && p.Scopes[scope] }

// keyEntry is one API key in the config JSON. Set KeySHA256 (hex SHA-256 of
// the key), or Key in plaintext for local development; Key is hashed on load
// and then dropped.
type keyEntry struct {
	Name      string   `json:"name"`
	Key       string   `json:"key,omitempty"`
	KeySHA256 string   `json:"key_sha256,omitempty"`
	Scopes    []string `json:"scopes"`
}

type keyRecord struct {
	hash      []byte
	principal *Principal
}

// KeyStore maps API keys to principals. It only keeps key hashes.
type KeyStore struct {
	records []keyRecord
}

// NewKeyStore builds a KeyStore from config entries.
func NewKeyStore(entries []keyEntry) (*KeyStore, error) {
	ks := &KeyStore{}
	for _, e := range entries {
		if e.Name == "" {
			return nil, errors.New("auth: key entry missing name")
		}
		var hash []byte
		switch {
		case e.KeySHA256 != "":
			h, err := hex.DecodeString(e.KeySHA256)
			if err != nil || len(h) != sha256.Size {
				return nil, fmt.Errorf("auth: invalid key_sha256 for %q", e.Name)
			}
			hash = h
		case e.Key != "":
			h := sha256.Sum256([]byte(e.Key))
			hash = h[:]
		default:
			return nil, fmt.Errorf("auth: key entry %q has neither key nor key_sha256", e.Name)
		}
		scopes := make(map[string]bool, len(e.Scopes))
		for _, s := range e.Scopes {
			scopes[strings.ToLower(strings.TrimSpace(s))] = true
		}
		ks.records = append(ks.records, keyRecord{
			hash:      hash,
			principal: &Principal{Name: e.Name, Scopes: scopes},
		})
	}
	return ks, nil
}

// LoadKeyStore loads keys from inlineJSON, or from filePath if that is empty.
// With neither, every request is denied.
func LoadKeyStore(inlineJSON, filePath string) (*KeyStore, error) {
	var data []byte
	switch {
	case inlineJSON != "":
		data = []byte(inlineJSON)
	case filePath != "":
		b, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("auth: read API keys file: %w", err)
		}
		data = b
	default:
		return NewKeyStore(nil)
	}
	var entries []keyEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("auth: parse API keys: %w", err)
	}
	return NewKeyStore(entries)
}

// Authenticate returns the principal for a bearer key, or nil.
func (ks *KeyStore) Authenticate(presented string) *Principal {
	sum := sha256.Sum256([]byte(presented))
	var match *Principal
	// Check every record, without stopping early, so timing doesn't reveal
	// whether or where a key matched.
	for _, r := range ks.records {
		if subtle.ConstantTimeCompare(r.hash, sum[:]) == 1 {
			match = r.principal
		}
	}
	return match
}
