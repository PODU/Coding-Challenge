// Package config reads the service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the service configuration.
type Config struct {
	Addr          string        // listen address
	DBPath        string        // SQLite path; ":memory:" or "dev" selects MemStore
	UseMemStore   bool          // derived: true when an in-memory store is requested
	MaxBytes      int64         // max paste size in bytes
	SweepInterval time.Duration // sweeper tick interval
	BaseURL       string        // base URL for building returned links
	MaxTTL        time.Duration // cap on user-requested TTL; 0 = unlimited
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		Addr:          ":8080",
		DBPath:        "pastebin.db",
		UseMemStore:   false,
		MaxBytes:      1 << 20, // 1 MiB
		SweepInterval: 60 * time.Second,
		BaseURL:       "http://localhost:8080",
		MaxTTL:        0,
	}
}

// FromEnv starts from Default and applies any environment variables that are
// set. A malformed value is an error.
func FromEnv() (Config, error) {
	c := Default()

	if v := os.Getenv("PASTEBIN_ADDR"); v != "" {
		c.Addr = v
	}
	if v := os.Getenv("PASTEBIN_DB"); v != "" {
		c.DBPath = v
	}
	if v := os.Getenv("PASTEBIN_BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("PASTEBIN_MAX_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return c, fmt.Errorf("PASTEBIN_MAX_BYTES: invalid value %q", v)
		}
		c.MaxBytes = n
	}
	if v := os.Getenv("PASTEBIN_SWEEP_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return c, fmt.Errorf("PASTEBIN_SWEEP_INTERVAL: invalid value %q", v)
		}
		c.SweepInterval = d
	}
	if v := os.Getenv("PASTEBIN_MAX_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 {
			return c, fmt.Errorf("PASTEBIN_MAX_TTL: invalid value %q", v)
		}
		c.MaxTTL = d
	}

	// A DB path of ":memory:", "mem", or "dev" selects the in-memory store.
	switch strings.ToLower(c.DBPath) {
	case ":memory:", "mem", "memory", "dev":
		c.UseMemStore = true
	}

	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	return c, nil
}
