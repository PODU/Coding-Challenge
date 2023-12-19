// Package urlparse is a small URL parser for http and https URLs. It doesn't
// use net/url; the point of the challenge is to do this by hand.
package urlparse

import (
	"errors"
	"strconv"
	"strings"
)

// URL is a parsed http or https URL.
type URL struct {
	Scheme string // "http" or "https"
	Host   string // IPv6 hosts are stored without brackets
	Port   string // filled in with the scheme default if absent
	Path   string // at least "/"
	Query  string // without the "?"
}

var (
	ErrEmptyURL     = errors.New("empty url")
	ErrNoScheme     = errors.New("missing scheme (expected http:// or https://)")
	ErrBadScheme    = errors.New("unsupported scheme (only http and https are supported)")
	ErrNoHost       = errors.New("missing host")
	ErrBadPort      = errors.New("invalid port")
	ErrUnclosedIPv6 = errors.New("unterminated IPv6 literal")
)

// DefaultPort returns 443 for https and 80 otherwise.
func DefaultPort(scheme string) string {
	if scheme == "https" {
		return "443"
	}
	return "80"
}

// RequestTarget is the path and query for the request line.
func (u URL) RequestTarget() string {
	if u.Query != "" {
		return u.Path + "?" + u.Query
	}
	return u.Path
}

// HostHeader returns the Host header value. The port is left out when it is
// the scheme default.
func (u URL) HostHeader() string {
	host := u.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if u.Port != DefaultPort(u.Scheme) {
		return host + ":" + u.Port
	}
	return host
}

// DialAddress returns host:port for net.Dial.
func (u URL) DialAddress() string {
	host := u.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return host + ":" + u.Port
}

// String is used when tracing redirects.
func (u URL) String() string {
	var b strings.Builder
	b.WriteString(u.Scheme)
	b.WriteString("://")
	b.WriteString(u.HostHeader())
	b.WriteString(u.Path)
	if u.Query != "" {
		b.WriteString("?")
		b.WriteString(u.Query)
	}
	return b.String()
}

// ParseURL parses an http or https URL, filling in the default port and path.
func ParseURL(raw string) (URL, error) {
	var u URL
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return u, ErrEmptyURL
	}

	idx := strings.Index(raw, "://")
	if idx < 0 {
		return u, ErrNoScheme
	}
	u.Scheme = strings.ToLower(raw[:idx])
	if u.Scheme != "http" && u.Scheme != "https" {
		return u, ErrBadScheme
	}
	rest := raw[idx+3:]

	// The authority (host[:port]) runs up to the first '/', '?' or '#'.
	authEnd := strings.IndexAny(rest, "/?#")
	if authEnd < 0 {
		authEnd = len(rest)
	}
	authority := rest[:authEnd]
	remainder := rest[authEnd:]

	host, port, err := splitHostPort(authority, u.Scheme)
	if err != nil {
		return u, err
	}
	if host == "" {
		return u, ErrNoHost
	}
	u.Host = host
	u.Port = port

	// The fragment is never sent.
	if h := strings.IndexByte(remainder, '#'); h >= 0 {
		remainder = remainder[:h]
	}

	if q := strings.IndexByte(remainder, '?'); q >= 0 {
		u.Path = remainder[:q]
		u.Query = remainder[q+1:]
	} else {
		u.Path = remainder
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}

// splitHostPort splits "host[:port]" or "[v6addr]:port", using the scheme's
// default port when none is given.
func splitHostPort(authority, scheme string) (host, port string, err error) {
	if authority == "" {
		return "", "", nil
	}
	if authority[0] == '[' {
		end := strings.IndexByte(authority, ']')
		if end < 0 {
			return "", "", ErrUnclosedIPv6
		}
		host = authority[1:end]
		after := authority[end+1:]
		if after == "" {
			return host, DefaultPort(scheme), nil
		}
		if after[0] != ':' {
			return "", "", ErrBadPort
		}
		port, err = validatePort(after[1:])
		return host, port, err
	}

	if i := strings.LastIndexByte(authority, ':'); i >= 0 {
		host = authority[:i]
		port, err = validatePort(authority[i+1:])
		return host, port, err
	}
	return authority, DefaultPort(scheme), nil
}

func validatePort(p string) (string, error) {
	if p == "" {
		return "", ErrBadPort
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 0 || n > 65535 {
		return "", ErrBadPort
	}
	return p, nil
}
