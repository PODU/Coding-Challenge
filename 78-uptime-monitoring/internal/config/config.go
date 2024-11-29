// Package config loads and validates the YAML configuration, applies defaults,
// and resolves it into the domain types used by the rest of the service.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/example/uptime/internal/monitor"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration document.
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Storage  StorageConfig   `yaml:"storage"`
	Defaults Defaults        `yaml:"defaults"`
	Alerters []AlerterConfig `yaml:"alerters"`
	Monitors []MonitorConfig `yaml:"monitors"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Listen string `yaml:"listen"`
}

// StorageConfig selects and configures the Store.
type StorageConfig struct {
	Driver    string   `yaml:"driver"` // sqlite | memory
	DSN       string   `yaml:"dsn"`
	Retention Duration `yaml:"retention"`
}

// Defaults are applied to monitors that omit a field.
type Defaults struct {
	Interval         Duration     `yaml:"interval"`
	Timeout          Duration     `yaml:"timeout"`
	Method           string       `yaml:"method"`
	ExpectStatus     ExpectStatus `yaml:"expect_status"`
	LatencyThreshold Duration     `yaml:"latency_threshold"`
	AlertThreshold   int          `yaml:"alert_threshold"`
}

// AlerterConfig configures one alerter. Unused fields for a given type are ignored.
type AlerterConfig struct {
	Type string   `yaml:"type"` // log | webhook | email
	URL  string   `yaml:"url"`  // webhook
	SMTP string   `yaml:"smtp"` // email host:port
	From string   `yaml:"from"` // email
	To   []string `yaml:"to"`   // email
}

// MonitorConfig is a single monitor as written in YAML (before defaults).
type MonitorConfig struct {
	ID               string       `yaml:"id"`
	Name             string       `yaml:"name"`
	Kind             string       `yaml:"kind"`
	Target           string       `yaml:"target"`
	Interval         Duration     `yaml:"interval"`
	Timeout          Duration     `yaml:"timeout"`
	Method           string       `yaml:"method"`
	ExpectStatus     ExpectStatus `yaml:"expect_status"`
	LatencyThreshold Duration     `yaml:"latency_threshold"`
	AlertThreshold   int          `yaml:"alert_threshold"`
	Enabled          *bool        `yaml:"enabled"`
}

// Duration is a time.Duration that unmarshals from a YAML string like "30s".
type Duration time.Duration

// UnmarshalYAML parses durations from strings or plain numbers (seconds).
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	s := strings.TrimSpace(node.Value)
	if s == "" {
		*d = 0
		return nil
	}
	// A bare number means seconds.
	if n, err := strconv.Atoi(s); err == nil {
		*d = Duration(time.Duration(n) * time.Second)
		return nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

// ExpectStatus is a list of acceptable HTTP status ranges parsed from YAML
// entries like 200 or "200-399".
type ExpectStatus []monitor.StatusRange

// UnmarshalYAML parses a sequence of ints and/or "lo-hi" range strings.
func (e *ExpectStatus) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("expect_status must be a list, got %v", node.Tag)
	}
	out := make(ExpectStatus, 0, len(node.Content))
	for _, n := range node.Content {
		r, err := parseStatusRange(n.Value)
		if err != nil {
			return err
		}
		out = append(out, r)
	}
	*e = out
	return nil
}

func parseStatusRange(s string) (monitor.StatusRange, error) {
	s = strings.TrimSpace(s)
	if lo, hi, ok := strings.Cut(s, "-"); ok {
		l, err1 := strconv.Atoi(strings.TrimSpace(lo))
		h, err2 := strconv.Atoi(strings.TrimSpace(hi))
		if err1 != nil || err2 != nil {
			return monitor.StatusRange{}, fmt.Errorf("invalid status range %q", s)
		}
		if l > h || l < 100 || h > 599 {
			return monitor.StatusRange{}, fmt.Errorf("status range %q out of bounds", s)
		}
		return monitor.StatusRange{Lo: l, Hi: h}, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 100 || n > 599 {
		return monitor.StatusRange{}, fmt.Errorf("invalid status code %q", s)
	}
	return monitor.StatusRange{Lo: n, Hi: n}, nil
}

// Load reads, parses, and validates a config file, applying defaults. Unknown
// keys are rejected.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(raw)
}

// Parse parses and validates config bytes.
func Parse(raw []byte) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true) // reject unknown keys
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.applyServerDefaults()
	if _, err := c.BuildMonitors(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) applyServerDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = ":8080"
	}
	if c.Storage.Driver == "" {
		c.Storage.Driver = "sqlite"
	}
	if c.Storage.DSN == "" {
		c.Storage.DSN = "./uptime.db"
	}
	if c.Defaults.Interval == 0 {
		c.Defaults.Interval = Duration(60 * time.Second)
	}
	if c.Defaults.Timeout == 0 {
		c.Defaults.Timeout = Duration(10 * time.Second)
	}
	if c.Defaults.Method == "" {
		c.Defaults.Method = "HEAD"
	}
	if len(c.Defaults.ExpectStatus) == 0 {
		c.Defaults.ExpectStatus = ExpectStatus{{Lo: 200, Hi: 399}}
	}
	if c.Defaults.AlertThreshold == 0 {
		c.Defaults.AlertThreshold = 3
	}
}

// BuildMonitors applies defaults to each configured monitor and validates it.
// Parse calls it too, so bad monitors are reported when the config is loaded.
func (c *Config) BuildMonitors() ([]monitor.Monitor, error) {
	c.applyServerDefaults()
	seen := make(map[string]bool, len(c.Monitors))
	out := make([]monitor.Monitor, 0, len(c.Monitors))
	for i, mc := range c.Monitors {
		m, err := c.resolve(mc)
		if err != nil {
			return nil, fmt.Errorf("monitor[%d] %q: %w", i, mc.ID, err)
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("duplicate monitor id %q", m.ID)
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	return out, nil
}

func (c *Config) resolve(mc MonitorConfig) (monitor.Monitor, error) {
	m := monitor.Monitor{
		ID:               strings.TrimSpace(mc.ID),
		Name:             mc.Name,
		Target:           strings.TrimSpace(mc.Target),
		Method:           strings.ToUpper(strings.TrimSpace(mc.Method)),
		Interval:         time.Duration(mc.Interval),
		Timeout:          time.Duration(mc.Timeout),
		LatencyThreshold: time.Duration(mc.LatencyThreshold),
		ExpectStatus:     []monitor.StatusRange(mc.ExpectStatus),
		AlertThreshold:   mc.AlertThreshold,
		Enabled:          mc.Enabled == nil || *mc.Enabled,
	}
	if m.ID == "" {
		return m, fmt.Errorf("id is required")
	}
	if m.Name == "" {
		m.Name = m.ID
	}

	kind, err := monitor.ParseKind(mc.Kind)
	if err != nil {
		return m, err
	}
	m.Kind = kind

	if m.Interval == 0 {
		m.Interval = time.Duration(c.Defaults.Interval)
	}
	if m.Timeout == 0 {
		m.Timeout = time.Duration(c.Defaults.Timeout)
	}
	if m.AlertThreshold == 0 {
		m.AlertThreshold = c.Defaults.AlertThreshold
	}
	if m.Kind == monitor.KindHTTP {
		if m.Method == "" {
			m.Method = strings.ToUpper(c.Defaults.Method)
		}
		if len(m.ExpectStatus) == 0 {
			m.ExpectStatus = []monitor.StatusRange(c.Defaults.ExpectStatus)
		}
		if m.Method != "HEAD" && m.Method != "GET" {
			return m, fmt.Errorf("method must be HEAD or GET, got %q", m.Method)
		}
	}

	if m.Target == "" {
		return m, fmt.Errorf("target is required")
	}
	switch m.Kind {
	case monitor.KindHTTP:
		u, err := url.Parse(m.Target)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return m, fmt.Errorf("invalid http target %q (need http(s)://host)", m.Target)
		}
	case monitor.KindTCP:
		host, port, err := net.SplitHostPort(m.Target)
		if err != nil || host == "" || port == "" {
			return m, fmt.Errorf("invalid tcp target %q (need host:port)", m.Target)
		}
	}

	if m.Interval <= 0 {
		return m, fmt.Errorf("interval must be > 0")
	}
	if m.Timeout <= 0 {
		return m, fmt.Errorf("timeout must be > 0")
	}
	if m.Timeout >= m.Interval {
		return m, fmt.Errorf("timeout (%s) must be less than interval (%s)", m.Timeout, m.Interval)
	}
	return m, nil
}
