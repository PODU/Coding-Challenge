package config

import (
	"path/filepath"
	"testing"
)

func TestLoadValid(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "testdata", "config.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Topic != "hn-stories" {
		t.Errorf("topic: got %q", cfg.Topic)
	}
	if len(cfg.Categories) != 3 {
		t.Errorf("categories: got %v", cfg.Categories)
	}
	if cfg.Filter.MinScore != 50 {
		t.Errorf("min_score: got %d", cfg.Filter.MinScore)
	}
	if len(cfg.Filter.Keywords) != 3 {
		t.Errorf("keywords: got %v", cfg.Filter.Keywords)
	}
	if cfg.PollDuration().String() != "1m0s" {
		t.Errorf("poll duration: got %v", cfg.PollDuration())
	}
}

func TestLoadDefaultsOnEmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if cfg.Topic != "hn-stories" {
		t.Errorf("default topic: got %q", cfg.Topic)
	}
	if cfg.PollDuration().String() != "1m0s" {
		t.Errorf("default poll: got %v", cfg.PollDuration())
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join("..", "..", "testdata", "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("KAFKA_TOPIC", "custom-topic")
	t.Setenv("KAFKA_BROKERS", "a:9092, b:9092")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Topic != "custom-topic" {
		t.Errorf("env topic override failed: %q", cfg.Topic)
	}
	if len(cfg.Brokers) != 2 {
		t.Errorf("env brokers override failed: %v", cfg.Brokers)
	}
}

func TestPollDurationInvalid(t *testing.T) {
	c := Config{PollInterval: "garbage"}
	if c.PollDuration().String() != "1m0s" {
		t.Errorf("invalid poll should default to 60s, got %v", c.PollDuration())
	}
}
