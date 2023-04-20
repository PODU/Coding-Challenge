package config

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// stringList is a repeatable string flag (e.g. -backend a -backend b).
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// Load builds a validated Config from args (without the program name).
// Precedence is defaults, then the -config file, then flags that were set
// explicitly. Usage and flag errors go to output.
func Load(args []string, output io.Writer) (Config, error) {
	fs := flag.NewFlagSet("lb", flag.ContinueOnError)
	fs.SetOutput(output)

	var (
		listen         = fs.String("listen", DefaultListen, "Address:port to listen on")
		configPath     = fs.String("config", "", "Path to YAML config file (optional)")
		healthPath     = fs.String("health-path", DefaultHealthPath, "Health check request path")
		healthInterval = fs.Duration("health-interval", DefaultHealthInterval, "Interval between health check rounds")
		healthTimeout  = fs.Duration("health-timeout", DefaultHealthTimeout, "Per-probe timeout")
		strategy       = fs.String("strategy", DefaultStrategy, "Scheduling strategy: round_robin")
		logLevel       = fs.String("log-level", DefaultLogLevel, "Log level: debug|info|warn|error")
		backends       stringList
	)
	fs.Var(&backends, "backend", "Backend URL; repeatable (e.g. -backend http://a:8080)")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Defaults()
	if *configPath != "" {
		var err error
		cfg, err = LoadFile(*configPath, cfg)
		if err != nil {
			return Config{}, err
		}
	}

	// Only flags the user actually passed should override the file.
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if set["listen"] {
		cfg.Listen = *listen
	}
	if set["health-path"] {
		cfg.HealthPath = *healthPath
	}
	if set["health-interval"] {
		cfg.HealthInterval = *healthInterval
	}
	if set["health-timeout"] {
		cfg.HealthTimeout = *healthTimeout
	}
	if set["strategy"] {
		cfg.Strategy = *strategy
	}
	if set["log-level"] {
		cfg.LogLevel = *logLevel
	}
	// -backend replaces the file's list rather than adding to it.
	if set["backend"] {
		cfg.Backends = []string(backends)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// LogLevelValue maps LogLevel to a slog level. ok is false for unknown names,
// in which case level is info.
func (c Config) LogLevelValue() (level int, ok bool) {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return -4, true
	case "info":
		return 0, true
	case "warn", "warning":
		return 4, true
	case "error":
		return 8, true
	default:
		return 0, false
	}
}
