package httpmsg

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Response is a parsed HTTP response.
type Response struct {
	Proto          string
	StatusCode     int
	Reason         string // may be empty
	Headers        Headers
	Body           []byte // already de-chunked
	RawStatusLine  string // without CRLF, for -i and -v
	RawHeaderLines []string
}

// Headers keeps header fields in the order they arrived.
type Headers []Header

// Get returns the first value for name, ignoring case.
func (h Headers) Get(name string) (string, bool) {
	for _, f := range h {
		if strings.EqualFold(f.Name, name) {
			return f.Value, true
		}
	}
	return "", false
}

var (
	ErrBadStatusLine = errors.New("malformed status line")
	ErrBadChunk      = errors.New("malformed chunked body")
)

// ParseResponse reads a whole response from r. For a HEAD request (headOnly)
// the body is not read, whatever Content-Length says.
func ParseResponse(r *bufio.Reader, headOnly bool) (*Response, error) {
	resp := &Response{}

	statusLine, err := readLine(r)
	if err != nil {
		return nil, fmt.Errorf("reading status line: %w", err)
	}
	resp.RawStatusLine = statusLine
	proto, code, reason, err := parseStatusLine(statusLine)
	if err != nil {
		return nil, err
	}
	resp.Proto = proto
	resp.StatusCode = code
	resp.Reason = reason

	for {
		line, err := readLine(r)
		if err != nil {
			return nil, fmt.Errorf("reading headers: %w", err)
		}
		if line == "" {
			break
		}
		resp.RawHeaderLines = append(resp.RawHeaderLines, line)
		name, value, ok := splitHeaderLine(line)
		if !ok {
			continue // skip malformed lines rather than fail
		}
		resp.Headers = append(resp.Headers, Header{Name: name, Value: value})
	}

	if headOnly || !bodyAllowed(code) {
		return resp, nil
	}

	body, err := readBody(r, resp.Headers)
	if err != nil {
		return nil, err
	}
	resp.Body = body
	return resp, nil
}

// bodyAllowed is false for 1xx, 204 and 304, which never have a body.
func bodyAllowed(code int) bool {
	if code >= 100 && code < 200 {
		return false
	}
	return code != 204 && code != 304
}

func readBody(r *bufio.Reader, headers Headers) ([]byte, error) {
	if te, ok := headers.Get("Transfer-Encoding"); ok && containsToken(te, "chunked") {
		return readChunked(r)
	}
	if cl, ok := headers.Get("Content-Length"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(cl))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid Content-Length %q", cl)
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, fmt.Errorf("reading body: %w", err)
		}
		return buf, nil
	}
	// No length given: read until the server closes (we sent Connection: close).
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	return buf, nil
}

func readChunked(r *bufio.Reader) ([]byte, error) {
	var out []byte
	for {
		sizeLine, err := readLine(r)
		if err != nil {
			return nil, fmt.Errorf("reading chunk size: %w", err)
		}
		// A chunk-size may carry chunk extensions after ';'.
		if i := strings.IndexByte(sizeLine, ';'); i >= 0 {
			sizeLine = sizeLine[:i]
		}
		sizeLine = strings.TrimSpace(sizeLine)
		size, err := strconv.ParseInt(sizeLine, 16, 64)
		if err != nil || size < 0 {
			return nil, ErrBadChunk
		}
		if size == 0 {
			// Skip any trailer headers.
			for {
				trailer, err := readLine(r)
				if err != nil {
					return nil, fmt.Errorf("reading chunk trailer: %w", err)
				}
				if trailer == "" {
					break
				}
			}
			return out, nil
		}
		chunk := make([]byte, size)
		if _, err := io.ReadFull(r, chunk); err != nil {
			return nil, fmt.Errorf("reading chunk data: %w", err)
		}
		out = append(out, chunk...)
		if _, err := readLine(r); err != nil {
			return nil, fmt.Errorf("reading chunk terminator: %w", err)
		}
	}
}

func parseStatusLine(line string) (proto string, code int, reason string, err error) {
	// "HTTP/1.1 200 OK" or "HTTP/1.1 200".
	first := strings.IndexByte(line, ' ')
	if first < 0 {
		return "", 0, "", ErrBadStatusLine
	}
	proto = line[:first]
	rest := strings.TrimLeft(line[first+1:], " ")
	codeStr := rest
	if second := strings.IndexByte(rest, ' '); second >= 0 {
		codeStr = rest[:second]
		reason = rest[second+1:]
	}
	if !strings.HasPrefix(proto, "HTTP/") {
		return "", 0, "", ErrBadStatusLine
	}
	code, err = strconv.Atoi(codeStr)
	if err != nil {
		return "", 0, "", ErrBadStatusLine
	}
	return proto, code, reason, nil
}

func splitHeaderLine(line string) (name, value string, ok bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	name = strings.TrimSpace(line[:i])
	value = strings.TrimSpace(line[i+1:])
	if name == "" {
		return "", "", false
	}
	return name, value, true
}

// readLine reads one line and strips the line ending. Bare LF is accepted.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		if err == io.EOF && line != "" {
			// Final line without a newline; accept it.
			return strings.TrimRight(line, "\r"), nil
		}
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

// containsToken checks a comma-separated header value such as "gzip, chunked".
func containsToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
