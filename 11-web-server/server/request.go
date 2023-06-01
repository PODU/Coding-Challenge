package server

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
)

// Byte caps to defend against slow-loris / memory-exhaustion style abuse.
const (
	maxRequestLineBytes = 8 * 1024  // 8 KiB request line
	maxHeaderBytes      = 64 * 1024 // 64 KiB of headers total
	maxHeaderCount      = 100
)

// errBadRequest signals malformed input that should yield a 400 response.
var errBadRequest = errors.New("bad request")

// Request is a hand-parsed HTTP/1.1 request. Only the parts this server needs
// are retained: the request line components and the headers.
type Request struct {
	Method  string
	Target  string // raw request target, e.g. "/index.html?x=1"
	Version string // e.g. "HTTP/1.1"
	Headers Headers
}

// Headers is a case-insensitive view over request headers. Keys are stored
// canonicalised to lower case.
type Headers map[string]string

// Get returns the value for the given header name (case-insensitive) and
// whether it was present.
func (h Headers) Get(name string) (string, bool) {
	v, ok := h[strings.ToLower(name)]
	return v, ok
}

// ParseRequest reads and parses a single HTTP request line plus its headers
// from r. The message body (if any) is left unread on the reader. A wrapped
// errBadRequest is returned for malformed input; io errors are returned as-is
// so the caller can distinguish a dropped connection from a 400.
func ParseRequest(r *bufio.Reader) (*Request, error) {
	line, err := readLine(r, maxRequestLineBytes)
	if err != nil {
		return nil, err
	}

	method, target, version, err := parseRequestLine(line)
	if err != nil {
		return nil, err
	}

	headers, err := parseHeaders(r)
	if err != nil {
		return nil, err
	}

	return &Request{
		Method:  method,
		Target:  target,
		Version: version,
		Headers: headers,
	}, nil
}

// parseRequestLine splits "METHOD SP TARGET SP VERSION" into its parts and
// validates each token.
func parseRequestLine(line string) (method, target, version string, err error) {
	parts := strings.Split(line, " ")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("%w: malformed request line %q", errBadRequest, line)
	}
	method, target, version = parts[0], parts[1], parts[2]

	if !isToken(method) {
		return "", "", "", fmt.Errorf("%w: invalid method %q", errBadRequest, method)
	}
	if target == "" || strings.ContainsRune(target, 0) {
		return "", "", "", fmt.Errorf("%w: invalid request target", errBadRequest)
	}
	if !strings.HasPrefix(version, "HTTP/") {
		return "", "", "", fmt.Errorf("%w: invalid http version %q", errBadRequest, version)
	}

	return method, target, version, nil
}

// parseHeaders reads header lines until the terminating blank line.
func parseHeaders(r *bufio.Reader) (Headers, error) {
	headers := make(Headers)
	total := 0

	for {
		line, err := readLine(r, maxRequestLineBytes)
		if err != nil {
			return nil, err
		}
		if line == "" {
			return headers, nil
		}

		total += len(line)
		if total > maxHeaderBytes || len(headers) >= maxHeaderCount {
			return nil, fmt.Errorf("%w: headers too large", errBadRequest)
		}

		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			return nil, fmt.Errorf("%w: malformed header %q", errBadRequest, line)
		}
		name := line[:colon]
		if !isToken(name) {
			return nil, fmt.Errorf("%w: invalid header name %q", errBadRequest, name)
		}
		value := strings.TrimSpace(line[colon+1:])

		key := strings.ToLower(name)
		if existing, ok := headers[key]; ok {
			// Repeated headers are joined with commas (RFC 7230 3.2.2).
			headers[key] = existing + ", " + value
		} else {
			headers[key] = value
		}
	}
}

// readLine reads a single CRLF- (or LF-) terminated line, returning it without
// the line ending. It enforces a byte cap to bound memory usage.
func readLine(r *bufio.Reader, limit int) (string, error) {
	var sb strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == '\n' {
			return strings.TrimSuffix(sb.String(), "\r"), nil
		}
		if sb.Len() >= limit {
			return "", fmt.Errorf("%w: line exceeds %d bytes", errBadRequest, limit)
		}
		sb.WriteByte(b)
	}
}

// isToken reports whether s is a non-empty HTTP token: visible ASCII with no
// separators (RFC 7230 3.2.6).
func isToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || c >= 0x7f {
			return false
		}
		switch c {
		case '(', ')', '<', '>', '@', ',', ';', ':', '\\', '"',
			'/', '[', ']', '?', '=', '{', '}':
			return false
		}
	}
	return true
}
