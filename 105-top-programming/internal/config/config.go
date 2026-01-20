// Package config loads service configuration from a YAML file with env overrides.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the service configuration shared by producer and api.
type Config struct {
	Brokers      []string `yaml:"brokers"`
	Topic        string   `yaml:"topic"`
	PollInterval string   `yaml:"poll_interval"`
	Categories   []string `yaml:"categories"`
	Filter       Filter   `yaml:"filter"`
}

// Filter is the consumer-side restriction applied to a deployment.
type Filter struct {
	Types    []string `yaml:"types"`
	Keywords []string `yaml:"keywords"`
	MinScore int      `yaml:"min_score"`
}

// Default returns a config with sane defaults applied.
func Default() Config {
	return Config{
		Brokers:      []string{"localhost:9092"},
		Topic:        "hn-stories",
		PollInterval: "60s",
		Categories:   []string{"top", "best"},
	}
}

// PollDuration parses PollInterval, defaulting to 60s when empty/invalid.
func (c Config) PollDuration() time.Duration {
	d, err := time.ParseDuration(c.PollInterval)
	if err != nil || d <= 0 {
		return 60 * time.Second
	}
	return d
}

// Load reads config from path, applies defaults, then env overrides.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("config: read %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
		}
	}
	applyEnv(&cfg)
	if cfg.Topic == "" {
		return Config{}, fmt.Errorf("config: topic must not be empty")
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		cfg.Brokers = splitCSV(v)
	}
	if v := os.Getenv("KAFKA_TOPIC"); v != "" {
		cfg.Topic = v
	}
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		cfg.PollInterval = v
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
