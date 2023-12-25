// Package urls produces N request targets from a single URL or from a file of
// URLs, cycling through the file if it holds fewer than N.
package urls

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// Provider yields exactly N URLs via At(i) for i in [0, N).
type Provider struct {
	urls []string // never empty
	n    int
}

// Single builds a provider that returns the same URL n times.
func Single(u string, n int) *Provider {
	return &Provider{urls: []string{u}, n: n}
}

// FromFile reads URLs from path (one per line; blank lines and lines whose
// first non-space rune is '#' are skipped) and builds a provider that cycles
// through them to produce n targets.
func FromFile(path string, n int) (*Provider, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	urls, err := parseLines(f)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, errors.New("url file contains no usable URLs")
	}
	return &Provider{urls: urls, n: n}, nil
}

func parseLines(r io.Reader) ([]string, error) {
	var out []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// N reports the number of targets the provider will produce.
func (p *Provider) N() int { return p.n }

// At returns the i-th target.
func (p *Provider) At(i int) string {
	return p.urls[i%len(p.urls)]
}

// All materializes the full ordered list of n targets.
func (p *Provider) All() []string {
	out := make([]string, p.n)
	for i := 0; i < p.n; i++ {
		out[i] = p.At(i)
	}
	return out
}
