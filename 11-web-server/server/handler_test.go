package server

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestRoot builds a temporary document root populated with a few files and
// returns its absolute path.
func newTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<h1>home</h1>")
	write("style.css", "body{}")
	write("sub/index.html", "<h1>sub</h1>")
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(root)
}

func req(method, target string) *Request {
	return &Request{Method: method, Target: target, Version: "HTTP/1.1", Headers: Headers{}}
}

func TestHandle_StatusCodes(t *testing.T) {
	root := newTestRoot(t)

	cases := []struct {
		name   string
		req    *Request
		status int
	}{
		{"root index", req("GET", "/"), 200},
		{"explicit file", req("GET", "/index.html"), 200},
		{"css file", req("GET", "/style.css"), 200},
		{"subdir index", req("GET", "/sub/"), 200},
		{"missing", req("GET", "/nope.html"), 404},
		{"dir no index", req("GET", "/empty/"), 404},
		{"traversal", req("GET", "/../../etc/passwd"), 403},
		{"post method", req("POST", "/"), 405},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := Handle(c.req, root)
			if resp.Status != c.status {
				t.Fatalf("status = %d, want %d", resp.Status, c.status)
			}
		})
	}
}

func TestHandle_ContentType(t *testing.T) {
	root := newTestRoot(t)

	cases := map[string]string{
		"/index.html": "text/html; charset=utf-8",
		"/style.css":  "text/css; charset=utf-8",
	}
	for target, want := range cases {
		t.Run(target, func(t *testing.T) {
			resp := Handle(req("GET", target), root)
			if got := resp.Headers["Content-Type"]; got != want {
				t.Fatalf("content-type = %q, want %q", got, want)
			}
		})
	}
}

func TestHandle_MethodNotAllowedSetsAllow(t *testing.T) {
	root := newTestRoot(t)
	resp := Handle(req("DELETE", "/"), root)
	if resp.Status != 405 {
		t.Fatalf("status = %d, want 405", resp.Status)
	}
	if resp.Headers["Allow"] != "GET, HEAD" {
		t.Fatalf("Allow = %q", resp.Headers["Allow"])
	}
}

func TestHandle_HeadNoBodyButLength(t *testing.T) {
	root := newTestRoot(t)
	get := Handle(req("GET", "/index.html"), root)
	head := Handle(req("HEAD", "/index.html"), root)

	if !head.HeadOnly {
		t.Fatal("HEAD response should be marked HeadOnly")
	}
	if head.Headers["Content-Length"] != get.Headers["Content-Length"] {
		t.Fatalf("HEAD content-length %q != GET %q",
			head.Headers["Content-Length"], get.Headers["Content-Length"])
	}
}

func TestContentType_UnknownExtension(t *testing.T) {
	if got := contentType("file.unknownext"); got != "application/octet-stream" {
		t.Fatalf("unknown ext content-type = %q", got)
	}
}
