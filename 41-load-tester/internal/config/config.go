// Package config parses and validates the command-line flags.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config is the validated configuration for one run.
type Config struct {
	URL     string        // -u target URL (empty if -f used)
	File    string        // -f file of URLs (empty if -u used)
	N       int           // -n total requests
	C       int           // -c concurrency (already clamped to <= N)
	Method  string        // -m HTTP method (upper-cased)
	Headers http.Header   // -H custom headers
	Body    string        // -d request body (already resolved from @file)
	Timeout time.Duration // -t per-request timeout
	JSON    bool          // -json emit JSON

	// Warnings are non-fatal problems found during validation, e.g. -c
	// being clamped to -n.
	Warnings []string
}

type headerList []string

func (h *headerList) String() string { return strings.Join(*h, ", ") }
func (h *headerList) Set(v string) error {
	*h = append(*h, v)
	return nil
}

var validMethods = map[string]bool{
	http.MethodGet:    true,
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
	http.MethodHead:   true,
}

// Parse builds a Config from the arguments after the program name.
func Parse(args []string) (Config, error) {
	return parse(args, os.ReadFile)
}

// parse takes readFile so tests can fake the -d @file lookup.
func parse(args []string, readFile func(string) ([]byte, error)) (Config, error) {
	fs := flag.NewFlagSet("loadtester", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller prints its own usage line
	var (
		u       = fs.String("u", "", "Target URL (required unless -f given)")
		file    = fs.String("f", "", "File of URLs (one per line; '#' comments allowed); cycled if fewer than -n")
		n       = fs.Int("n", 10, "Total number of requests to send")
		c       = fs.Int("c", 1, "Number of concurrent workers")
		method  = fs.String("m", http.MethodGet, "HTTP method: GET|POST|PUT|PATCH|DELETE|HEAD")
		body    = fs.String("d", "", "Request body data (string, or @file to read from file)")
		timeout = fs.Duration("t", 30*time.Second, "Per-request timeout, e.g. 5s, 500ms")
		asJSON  = fs.Bool("json", false, "Emit summary as JSON instead of a table")
		headers headerList
	)
	fs.Var(&headers, "H", "Custom header 'Key: Value' (repeatable)")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Config{
		URL:     *u,
		File:    *file,
		N:       *n,
		C:       *c,
		Method:  strings.ToUpper(strings.TrimSpace(*method)),
		Body:    *body,
		Timeout: *timeout,
		JSON:    *asJSON,
		Headers: http.Header{},
	}

	if cfg.URL == "" && cfg.File == "" {
		return Config{}, errors.New("one of -u <url> or -f <file> is required")
	}
	if cfg.URL != "" && cfg.File != "" {
		return Config{}, errors.New("-u and -f are mutually exclusive")
	}

	// URLs from a file are only checked when they are requested.
	if cfg.URL != "" {
		if err := validateURL(cfg.URL); err != nil {
			return Config{}, err
		}
	}

	if cfg.N < 1 {
		return Config{}, fmt.Errorf("-n must be >= 1, got %d", cfg.N)
	}
	if cfg.C < 1 {
		return Config{}, fmt.Errorf("-c must be >= 1, got %d", cfg.C)
	}
	if cfg.C > cfg.N {
		cfg.Warnings = append(cfg.Warnings,
			fmt.Sprintf("concurrency -c %d exceeds -n %d; clamping to %d", cfg.C, cfg.N, cfg.N))
		cfg.C = cfg.N
	}

	if !validMethods[cfg.Method] {
		return Config{}, fmt.Errorf("unsupported HTTP method %q", cfg.Method)
	}

	if cfg.Timeout <= 0 {
		return Config{}, fmt.Errorf("-t must be > 0, got %s", cfg.Timeout)
	}

	for _, raw := range headers {
		k, v, ok := strings.Cut(raw, ":")
		if !ok || strings.TrimSpace(k) == "" {
			return Config{}, fmt.Errorf("malformed header %q (want 'Key: Value')", raw)
		}
		cfg.Headers.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	if strings.HasPrefix(cfg.Body, "@") {
		path := cfg.Body[1:]
		data, err := readFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("reading body file %q: %w", path, err)
		}
		cfg.Body = string(data)
	}

	return cfg, nil
}

func validateURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("invalid URL scheme %q (want http or https)", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid URL %q: missing host", raw)
	}
	return nil
}
