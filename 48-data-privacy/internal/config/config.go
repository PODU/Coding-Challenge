// Package config reads the service configuration from environment variables.
// There are no built-in secrets: the master key and API keys must be supplied.
package config

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"datavault/internal/crypto"
)

// Config is the service configuration.
type Config struct {
	Addr         string
	StoreBackend string // memory, bolt or redis
	BoltPath     string
	RedisAddr    string
	RedisPass    string
	RedisDB      int

	MasterKey []byte // 32 bytes
	DedupKey  []byte // 32 bytes; derived from MasterKey if not set

	APIKeysFile string
	APIKeysJSON string // API_KEYS; wins over APIKeysFile

	TLSCert string
	TLSKey  string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads and validates the configuration.
func Load() (*Config, error) {
	c := &Config{
		Addr:         getenv("ADDR", ":8443"),
		StoreBackend: strings.ToLower(getenv("STORE_BACKEND", "bolt")),
		BoltPath:     getenv("BOLT_PATH", "vault.db"),
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		RedisPass:    os.Getenv("REDIS_PASSWORD"),
		APIKeysFile:  os.Getenv("API_KEYS_FILE"),
		APIKeysJSON:  os.Getenv("API_KEYS"),
		TLSCert:      os.Getenv("TLS_CERT"),
		TLSKey:       os.Getenv("TLS_KEY"),
	}

	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		db, err := strconv.Atoi(dbStr)
		if err != nil {
			return nil, fmt.Errorf("config: REDIS_DB: %w", err)
		}
		c.RedisDB = db
	}

	key, err := loadMasterKey()
	if err != nil {
		return nil, err
	}
	c.MasterKey = key

	if d := os.Getenv("VAULT_DEDUP_KEY"); d != "" {
		dk, err := base64.StdEncoding.DecodeString(d)
		if err != nil {
			return nil, fmt.Errorf("config: VAULT_DEDUP_KEY base64: %w", err)
		}
		if len(dk) != crypto.KeySize {
			return nil, fmt.Errorf("config: VAULT_DEDUP_KEY must decode to %d bytes", crypto.KeySize)
		}
		c.DedupKey = dk
	} else {
		// Derive it so one env var is enough for local use, while still keeping
		// the HMAC key separate from the encryption key.
		sum := sha256.Sum256(append([]byte("dedup:"), c.MasterKey...))
		c.DedupKey = sum[:]
	}

	return c, nil
}

// loadMasterKey decodes VAULT_MASTER_KEY. There is deliberately no default.
func loadMasterKey() ([]byte, error) {
	raw := os.Getenv("VAULT_MASTER_KEY")
	if raw == "" {
		return nil, errors.New("config: VAULT_MASTER_KEY is required (base64 of 32 bytes); generate one with `vault genkey`")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("config: VAULT_MASTER_KEY base64: %w", err)
	}
	if len(key) != crypto.KeySize {
		return nil, fmt.Errorf("config: VAULT_MASTER_KEY must decode to %d bytes, got %d", crypto.KeySize, len(key))
	}
	return key, nil
}
