// Package filter holds the proxy's host blocklist and banned-word list. A
// Filter is loaded at startup and only read afterwards, so it can be shared
// between connections.
package filter

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// Filter holds the loaded blocklists. Use New to create one.
type Filter struct {
	forbiddenHosts map[string]struct{}
	bannedWords    []string // lower-cased, non-empty
}

// New returns a Filter that blocks nothing.
func New() *Filter {
	return &Filter{forbiddenHosts: make(map[string]struct{})}
}

// HasBannedWords reports whether response bodies need scanning at all.
func (f *Filter) HasBannedWords() bool {
	return len(f.bannedWords) > 0
}

// HostForbidden reports whether host or one of its parent domains is blocked:
// "example.com" blocks "www.example.com" but not "notexample.com". Any port
// is ignored.
func (f *Filter) HostForbidden(host string) bool {
	if len(f.forbiddenHosts) == 0 {
		return false
	}
	host = normalizeHost(host)
	if host == "" {
		return false
	}
	if _, ok := f.forbiddenHosts[host]; ok {
		return true
	}
	for i := 0; i < len(host); i++ {
		if host[i] == '.' {
			if _, ok := f.forbiddenHosts[host[i+1:]]; ok {
				return true
			}
		}
	}
	return false
}

// BodyHasBanned returns the first banned word found in body, ignoring case.
func (f *Filter) BodyHasBanned(body []byte) (string, bool) {
	if len(f.bannedWords) == 0 || len(body) == 0 {
		return "", false
	}
	lower := strings.ToLower(string(body))
	for _, w := range f.bannedWords {
		if strings.Contains(lower, w) {
			return w, true
		}
	}
	return "", false
}

// normalizeHost lower-cases host and strips the port and any IPv6 brackets.
func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "[") {
		if end := strings.IndexByte(host, ']'); end >= 0 {
			return host[1:end]
		}
		return host
	}
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		// A bare IPv6 address has several colons and no port.
		if strings.IndexByte(host, ':') == i {
			return host[:i]
		}
	}
	return host
}

// LoadHosts loads a forbidden-hosts file. The bool reports whether the file
// existed; a missing file is not an error.
func (f *Filter) LoadHosts(path string) (bool, error) {
	lines, existed, err := readList(path)
	if err != nil {
		return existed, err
	}
	for _, l := range lines {
		f.forbiddenHosts[normalizeHost(l)] = struct{}{}
	}
	return existed, nil
}

// LoadWords loads a banned-words file into f. A missing file is not an error.
func (f *Filter) LoadWords(path string) (bool, error) {
	lines, existed, err := readList(path)
	if err != nil {
		return existed, err
	}
	for _, l := range lines {
		w := strings.ToLower(strings.TrimSpace(l))
		if w != "" {
			f.bannedWords = append(f.bannedWords, w)
		}
	}
	return existed, nil
}

// readList returns the non-blank, non-comment lines of path, trimmed.
// A missing file gives (nil, false, nil).
func readList(path string) (lines []string, existed bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return nil, true, err
	}
	return lines, true, nil
}
