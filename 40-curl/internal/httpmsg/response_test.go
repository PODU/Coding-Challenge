package httpmsg

import (
	"bufio"
	"strings"
	"testing"
)

func parse(t *testing.T, raw string, headOnly bool) *Response {
	t.Helper()
	r := bufio.NewReader(strings.NewReader(raw))
	resp, err := ParseResponse(r, headOnly)
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	return resp
}

func TestParseContentLength(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 5\r\n" +
		"\r\n" +
		"hello"
	resp := parse(t, raw, false)
	if resp.StatusCode != 200 || resp.Reason != "OK" {
		t.Errorf("status = %d %q", resp.StatusCode, resp.Reason)
	}
	if string(resp.Body) != "hello" {
		t.Errorf("body = %q", resp.Body)
	}
	if ct, _ := resp.Headers.Get("content-type"); ct != "text/plain" {
		t.Errorf("content-type = %q", ct)
	}
}

func TestParseChunked(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"5\r\nhello\r\n" +
		"6\r\n world\r\n" +
		"0\r\n\r\n"
	resp := parse(t, raw, false)
	if string(resp.Body) != "hello world" {
		t.Errorf("body = %q, want %q", resp.Body, "hello world")
	}
}

func TestParseChunkedWithExtensionAndTrailer(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		"4;foo=bar\r\nWiki\r\n" +
		"5\r\npedia\r\n" +
		"0\r\n" +
		"X-Trailer: end\r\n" +
		"\r\n"
	resp := parse(t, raw, false)
	if string(resp.Body) != "Wikipedia" {
		t.Errorf("body = %q, want %q", resp.Body, "Wikipedia")
	}
}

func TestParseReadToEOF(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\n" +
		"\r\n" +
		"unframed body content"
	resp := parse(t, raw, false)
	if string(resp.Body) != "unframed body content" {
		t.Errorf("body = %q", resp.Body)
	}
}

func TestParseHEADNoBody(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\n" +
		"Content-Length: 100\r\n" +
		"\r\n"
	resp := parse(t, raw, true)
	if resp.Body != nil {
		t.Errorf("HEAD body = %q, want nil", resp.Body)
	}
}

func TestParseStatusNoReason(t *testing.T) {
	raw := "HTTP/1.1 204\r\n\r\n"
	resp := parse(t, raw, false)
	if resp.StatusCode != 204 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if resp.Reason != "" {
		t.Errorf("reason = %q, want empty", resp.Reason)
	}
	if resp.Body != nil {
		t.Errorf("204 body = %q, want nil", resp.Body)
	}
}

func TestParseBareLF(t *testing.T) {
	// Tolerate bare \n line endings.
	raw := "HTTP/1.1 200 OK\n" +
		"Content-Length: 2\n" +
		"\n" +
		"hi"
	resp := parse(t, raw, false)
	if string(resp.Body) != "hi" {
		t.Errorf("body = %q", resp.Body)
	}
}

func TestParseDuplicateHeaders(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\n" +
		"Set-Cookie: a=1\r\n" +
		"Set-Cookie: b=2\r\n" +
		"Content-Length: 0\r\n" +
		"\r\n"
	resp := parse(t, raw, false)
	count := 0
	for _, h := range resp.Headers {
		if strings.EqualFold(h.Name, "Set-Cookie") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("Set-Cookie count = %d, want 2", count)
	}
	// Get returns the first.
	if v, _ := resp.Headers.Get("Set-Cookie"); v != "a=1" {
		t.Errorf("first Set-Cookie = %q, want a=1", v)
	}
}

func TestParseBadStatusLine(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("garbage\r\n\r\n"))
	if _, err := ParseResponse(r, false); err == nil {
		t.Error("expected error for bad status line")
	}
}
