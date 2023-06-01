package server

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

func parse(t *testing.T, raw string) (*Request, error) {
	t.Helper()
	return ParseRequest(bufio.NewReader(strings.NewReader(raw)))
}

func TestParseRequest_Valid(t *testing.T) {
	raw := "GET /index.html HTTP/1.1\r\nHost: example.com\r\nUser-Agent: test\r\n\r\n"
	req, err := parse(t, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Method != "GET" || req.Target != "/index.html" || req.Version != "HTTP/1.1" {
		t.Fatalf("bad request line parse: %+v", req)
	}
	if v, ok := req.Headers.Get("host"); !ok || v != "example.com" {
		t.Fatalf("host header = %q, %v", v, ok)
	}
}

func TestParseRequest_LFOnly(t *testing.T) {
	// Tolerate bare LF line endings.
	req, err := parse(t, "GET / HTTP/1.1\nHost: x\n\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Target != "/" {
		t.Fatalf("target = %q", req.Target)
	}
}

func TestParseRequest_CaseInsensitiveHeaders(t *testing.T) {
	req, err := parse(t, "GET / HTTP/1.1\r\nCONTENT-type: text/html\r\n\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := req.Headers.Get("Content-Type"); !ok || v != "text/html" {
		t.Fatalf("case-insensitive lookup failed: %q %v", v, ok)
	}
}

func TestParseRequest_DuplicateHeaders(t *testing.T) {
	req, err := parse(t, "GET / HTTP/1.1\r\nX-Tag: a\r\nX-Tag: b\r\n\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, _ := req.Headers.Get("x-tag"); v != "a, b" {
		t.Fatalf("duplicate header combine = %q", v)
	}
}

func TestParseRequest_Malformed(t *testing.T) {
	cases := map[string]string{
		"missing version":   "GET /\r\n\r\n",
		"empty method":      " / HTTP/1.1\r\n\r\n",
		"bad version token": "GET / FTP/1.1\r\n\r\n",
		"bad header":        "GET / HTTP/1.1\r\nnocolon\r\n\r\n",
		"empty header name": "GET / HTTP/1.1\r\n: value\r\n\r\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parse(t, raw); !errors.Is(err, errBadRequest) {
				t.Fatalf("expected errBadRequest, got %v", err)
			}
		})
	}
}

func TestParseRequest_TruncatedIsEOF(t *testing.T) {
	// Connection closed before the blank line: should surface an io error, not
	// a bad-request error.
	_, err := parse(t, "GET / HTTP/1.1\r\nHost: x\r\n")
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestParseRequest_RequestLineTooLong(t *testing.T) {
	raw := "GET /" + strings.Repeat("a", maxRequestLineBytes+10) + " HTTP/1.1\r\n\r\n"
	if _, err := parse(t, raw); !errors.Is(err, errBadRequest) {
		t.Fatalf("expected errBadRequest, got %v", err)
	}
}
