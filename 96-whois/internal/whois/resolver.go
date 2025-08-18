package whois

import (
	"net"
	"strings"
)

// ianaServer is where every lookup starts. Its "refer:" field points to the
// right registry, so we don't need a table of TLD servers.
const ianaServer = "whois.iana.org"

// QueryType is the kind of thing being looked up.
type QueryType int

const (
	TypeDomain QueryType = iota
	TypeIPv4
	TypeIPv6
	TypeASN // e.g. AS15169
)

// String returns the name used in log output.
func (t QueryType) String() string {
	switch t {
	case TypeIPv4:
		return "ipv4"
	case TypeIPv6:
		return "ipv6"
	case TypeASN:
		return "asn"
	default:
		return "domain"
	}
}

// Classify works out what kind of query this is.
func Classify(query string) QueryType {
	q := strings.TrimSpace(query)
	if isASN(q) {
		return TypeASN
	}
	if ip := net.ParseIP(q); ip != nil {
		if ip.To4() != nil {
			return TypeIPv4
		}
		return TypeIPv6
	}
	return TypeDomain
}

// isASN reports whether q looks like "AS" followed by digits.
func isASN(q string) bool {
	u := strings.ToUpper(strings.TrimSpace(q))
	if !strings.HasPrefix(u, "AS") {
		return false
	}
	digits := u[2:]
	if digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// FormatQuery returns the request line (without CRLF) for server. Most
// servers take the bare query, but some need a prefix:
//
//   - Verisign needs "domain example.com", or it returns every partial match
//   - ARIN uses "n + <ip>" for networks and "a + <asn>" for AS numbers
func FormatQuery(server, query string, t QueryType) string {
	host := strings.ToLower(strings.TrimSpace(server))
	q := strings.TrimSpace(query)

	switch {
	case strings.Contains(host, "verisign-grs.com") && t == TypeDomain:
		return "domain " + q
	case strings.Contains(host, "arin.net") && (t == TypeIPv4 || t == TypeIPv6):
		return "n + " + q
	case strings.Contains(host, "arin.net") && t == TypeASN:
		return "a + " + q
	}
	return q
}
