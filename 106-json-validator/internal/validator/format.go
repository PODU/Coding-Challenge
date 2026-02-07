package validator

import (
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sync"
	"time"
)

var (
	patternMu    sync.Mutex
	patternCache = map[string]*regexp.Regexp{}
)

// compilePattern compiles and caches a pattern. Patterns run on Go's RE2
// engine rather than ECMA-262, and like JSON Schema they match anywhere in the
// string unless anchored.
func compilePattern(pat string) (*regexp.Regexp, error) {
	patternMu.Lock()
	defer patternMu.Unlock()
	if re, ok := patternCache[pat]; ok {
		return re, nil
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, err
	}
	patternCache[pat] = re
	return re, nil
}

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// checkFormat reports whether s matches the named format. Unknown formats
// always pass, since JSON Schema says they must not cause a failure.
func checkFormat(format, s string) bool {
	switch format {
	case "email":
		// ParseAddress also accepts "Name <addr>", so insist on the bare address.
		addr, err := mail.ParseAddress(s)
		return err == nil && addr.Address == s
	case "date":
		_, err := time.Parse("2006-01-02", s)
		return err == nil
	case "date-time":
		if _, err := time.Parse(time.RFC3339, s); err == nil {
			return true
		}
		_, err := time.Parse(time.RFC3339Nano, s)
		return err == nil
	case "ipv4":
		ip := net.ParseIP(s)
		return ip != nil && ip.To4() != nil
	case "uri":
		u, err := url.Parse(s)
		return err == nil && u.IsAbs()
	case "uuid":
		return uuidRE.MatchString(s)
	default:
		return true
	}
}
