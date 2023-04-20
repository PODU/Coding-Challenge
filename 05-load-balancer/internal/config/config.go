// Package config loads settings from flags and an optional YAML file. Flags
// given on the command line win over the file.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultListen          = ":8080"
	DefaultHealthPath      = "/health"
	DefaultHealthInterval  = 10 * time.Second
	DefaultHealthTimeout   = 2 * time.Second
	DefaultStrategy        = "round_robin"
	DefaultLogLevel        = "info"
	DefaultDialTimeout     = 5 * time.Second
	DefaultRespHdrTimeout  = 30 * time.Second
	DefaultShutdownTimeout = 15 * time.Second
)

// Config is the resolved runtime configuration.
type Config struct {
	Listen          string
	Backends        []string
	HealthPath      string
	HealthInterval  time.Duration
	HealthTimeout   time.Duration
	Strategy        string
	LogLevel        string
	DialTimeout     time.Duration
	RespHdrTimeout  time.Duration
	ShutdownTimeout time.Duration
}

type fileConfig struct {
	Listen   string   `yaml:"listen"`
	Backends []string `yaml:"backends"`
	Health   struct {
		Path     string `yaml:"path"`
		Interval string `yaml:"interval"`
		Timeout  string `yaml:"timeout"`
	} `yaml:"health"`
	Strategy string `yaml:"strategy"`
	LogLevel string `yaml:"log_level"`
	Proxy    struct {
		DialTimeout           string `yaml:"dial_timeout"`
		ResponseHeaderTimeout string `yaml:"response_header_timeout"`
		ShutdownTimeout       string `yaml:"shutdown_timeout"`
	} `yaml:"proxy"`
}

// Defaults returns a Config populated with built-in defaults (no backends).
func Defaults() Config {
	return Config{
		Listen:          DefaultListen,
		HealthPath:      DefaultHealthPath,
		HealthInterval:  DefaultHealthInterval,
		HealthTimeout:   DefaultHealthTimeout,
		Strategy:        DefaultStrategy,
		LogLevel:        DefaultLogLevel,
		DialTimeout:     DefaultDialTimeout,
		RespHdrTimeout:  DefaultRespHdrTimeout,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

// LoadFile overlays a YAML config file onto base. Fields missing from the
// file keep their value from base.
func LoadFile(path string, base Config) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return base, fmt.Errorf("read config file: %w", err)
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return base, fmt.Errorf("parse config file %q: %w", path, err)
	}

	cfg := base
	if fc.Listen != "" {
		cfg.Listen = fc.Listen
	}
	if len(fc.Backends) > 0 {
		cfg.Backends = append([]string(nil), fc.Backends...)
	}
	if fc.Health.Path != "" {
		cfg.HealthPath = fc.Health.Path
	}
	if fc.Strategy != "" {
		cfg.Strategy = fc.Strategy
	}
	if fc.LogLevel != "" {
		cfg.LogLevel = fc.LogLevel
	}

	for _, d := range []struct {
		raw   string
		field *time.Duration
		name  string
	}{
		{fc.Health.Interval, &cfg.HealthInterval, "health.interval"},
		{fc.Health.Timeout, &cfg.HealthTimeout, "health.timeout"},
		{fc.Proxy.DialTimeout, &cfg.DialTimeout, "proxy.dial_timeout"},
		{fc.Proxy.ResponseHeaderTimeout, &cfg.RespHdrTimeout, "proxy.response_header_timeout"},
		{fc.Proxy.ShutdownTimeout, &cfg.ShutdownTimeout, "proxy.shutdown_timeout"},
	} {
		if d.raw == "" {
			continue
		}
		v, err := time.ParseDuration(d.raw)
		if err != nil {
			return base, fmt.Errorf("parse %s %q: %w", d.name, d.raw, err)
		}
		*d.field = v
	}

	return cfg, nil
}

// Validate reports the first problem with c, if any.
func (c Config) Validate() error {
	if len(c.Backends) == 0 {
		return errors.New("at least one backend is required")
	}
	seen := make(map[string]struct{}, len(c.Backends))
	for _, raw := range c.Backends {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid backend URL %q: %w", raw, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("backend URL %q must include scheme and host (e.g. http://host:port)", raw)
		}
		if _, dup := seen[raw]; dup {
			return fmt.Errorf("duplicate backend URL %q", raw)
		}
		seen[raw] = struct{}{}
	}
	if c.Listen == "" {
		return errors.New("listen address must not be empty")
	}
	if c.HealthInterval <= 0 {
		return errors.New("health interval must be positive")
	}
	if c.HealthTimeout <= 0 {
		return errors.New("health timeout must be positive")
	}
	if c.Strategy != "round_robin" {
		return fmt.Errorf("unknown strategy %q (supported: round_robin)", c.Strategy)
	}
	return nil
}

// ParsedBackends parses the backend URLs. Run Validate first.
func (c Config) ParsedBackends() ([]*url.URL, error) {
	out := make([]*url.URL, 0, len(c.Backends))
	for _, raw := range c.Backends {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid backend URL %q: %w", raw, err)
		}
		out = append(out, u)
	}
	return out, nil
}
