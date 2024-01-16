// Package blocklist loads a set of domain names that the forwarder answers
// locally (NXDOMAIN or a sinkhole address) instead of forwarding upstream.
package blocklist

import (
	"bufio"
	"os"
	"strings"
)

// List is an immutable, case-insensitive set of blocked domains. A name matches
// if it equals a listed entry or is a subdomain of one (so blocking
// "example.com" also blocks "ads.example.com").
type List struct {
	set map[string]struct{}
}

// New builds a List from the given names (comments/blank entries are ignored).
func New(names []string) *List {
	l := &List{set: make(map[string]struct{}, len(names))}
	for _, n := range names {
		if name, ok := normalize(n); ok {
			l.set[name] = struct{}{}
		}
	}
	return l
}

// Load reads a newline-delimited blocklist file. Lines may be bare domains or
// "0.0.0.0 domain" / "127.0.0.1 domain" hosts-file style; the last whitespace
// field is taken as the domain. Blank lines and lines beginning with '#' are
// ignored.
func Load(path string) (*List, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var names []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		names = append(names, fields[len(fields)-1])
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return New(names), nil
}

func normalize(s string) (string, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, ".")
	if s == "" || strings.HasPrefix(s, "#") {
		return "", false
	}
	return s, true
}

// Blocked reports whether name (or any parent domain match) is blocked.
func (l *List) Blocked(name string) bool {
	if l == nil || len(l.set) == 0 {
		return false
	}
	n, ok := normalize(name)
	if !ok {
		return false
	}
	for {
		if _, found := l.set[n]; found {
			return true
		}
		i := strings.IndexByte(n, '.')
		if i < 0 {
			return false
		}
		n = n[i+1:]
	}
}

// Len returns the number of entries.
func (l *List) Len() int {
	if l == nil {
		return 0
	}
	return len(l.set)
}
