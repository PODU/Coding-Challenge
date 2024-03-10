package registry

import (
	"fmt"
	"net/url"
	"strings"
)

// Challenge is a parsed `WWW-Authenticate: Bearer ...` header.
type Challenge struct {
	Realm   string // token-endpoint URL, e.g. "https://auth.docker.io/token"
	Service string // e.g. "registry.docker.io"
	Scope   string // e.g. "repository:library/ubuntu:pull"
}

// ParseChallenge parses a header such as
//
//	Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/ubuntu:pull"
//
// Values may or may not be quoted.
func ParseChallenge(header string) (Challenge, error) {
	s := strings.TrimSpace(header)
	if s == "" {
		return Challenge{}, fmt.Errorf("empty WWW-Authenticate header")
	}
	if !strings.HasPrefix(strings.ToLower(s), "bearer") {
		return Challenge{}, fmt.Errorf("unsupported auth scheme in %q (want Bearer)", header)
	}
	s = strings.TrimSpace(s[len("bearer"):])

	params := parseAuthParams(s)
	ch := Challenge{
		Realm:   params["realm"],
		Service: params["service"],
		Scope:   params["scope"],
	}
	if ch.Realm == "" {
		return Challenge{}, fmt.Errorf("WWW-Authenticate header missing realm: %q", header)
	}
	return ch, nil
}

// TokenRequestURL returns the URL to request an anonymous token from.
func TokenRequestURL(c Challenge) (string, error) {
	u, err := url.Parse(c.Realm)
	if err != nil {
		return "", fmt.Errorf("invalid token realm %q: %w", c.Realm, err)
	}
	q := u.Query()
	if c.Service != "" {
		q.Set("service", c.Service)
	}
	if c.Scope != "" {
		q.Set("scope", c.Scope)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// PullScope returns the token scope for pulling repository.
func PullScope(repository string) string {
	return "repository:" + repository + ":pull"
}

// parseAuthParams parses comma-separated key=value pairs. Quoted values may
// contain commas.
func parseAuthParams(s string) map[string]string {
	m := make(map[string]string)
	var key, val strings.Builder
	inKey := true
	inQuotes := false

	flush := func() {
		k := strings.TrimSpace(key.String())
		if k != "" {
			m[k] = val.String()
		}
		key.Reset()
		val.Reset()
		inKey = true
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
		case c == '=' && !inQuotes && inKey:
			inKey = false
		case c == ',' && !inQuotes:
			flush()
		default:
			if inKey {
				key.WriteByte(c)
			} else {
				val.WriteByte(c)
			}
		}
	}
	flush()
	return m
}
