// Package config reads the server's settings from SMT_* environment
// variables. The defaults run with only the mock provider.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr            string // HTTP listen address
	DSN             string // SQLite file
	Workers         int    // publisher goroutines
	QueueCapacity   int
	SchedulerTick   time.Duration // how often to look for due posts
	MaxAttempts     int           // publish attempts before giving up
	EnabledTargets  []string      // any of mock, linkedin, x
	LinkedInToken   string
	LinkedInAuthor  string
	XToken          string
	ShutdownTimeout time.Duration
}

func Load() Config {
	return Config{
		Addr:            env("SMT_ADDR", ":8080"),
		DSN:             env("SMT_DSN", "social.db"),
		Workers:         envInt("SMT_WORKERS", 4),
		QueueCapacity:   envInt("SMT_QUEUE_CAPACITY", 256),
		SchedulerTick:   envDuration("SMT_SCHEDULER_TICK", 5*time.Second),
		MaxAttempts:     envInt("SMT_MAX_ATTEMPTS", 3),
		EnabledTargets:  envList("SMT_TARGETS", []string{"mock"}),
		LinkedInToken:   os.Getenv("SMT_LINKEDIN_TOKEN"),
		LinkedInAuthor:  os.Getenv("SMT_LINKEDIN_AUTHOR"),
		XToken:          os.Getenv("SMT_X_TOKEN"),
		ShutdownTimeout: envDuration("SMT_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envList(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
