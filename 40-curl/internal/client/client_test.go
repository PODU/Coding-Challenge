package client

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codingchallenges/cc-curl/internal/httpmsg"
	"github.com/codingchallenges/cc-curl/internal/trace"
	"github.com/codingchallenges/cc-curl/internal/urlparse"
)

// plainDialer returns a Dialer that opens a raw TCP connection to the URL's
// address (used against an httptest.Server).
func plainDialer() Dialer {
	return func(u urlparse.URL) (net.Conn, error) {
		return net.Dial("tcp", u.DialAddress())
	}
}

// tlsDialer returns a Dialer that opens a TLS connection skipping verification
// (used against an httptest.NewTLSServer).
func tlsDialer() Dialer {
	return func(u urlparse.URL) (net.Conn, error) {
		return tls.Dial("tcp", u.DialAddress(), &tls.Config{
			ServerName:         u.Host,
			InsecureSkipVerify: true,
		})
	}
}

func mustURL(t *testing.T, raw string) urlparse.URL {
	t.Helper()
	u, err := urlparse.ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL(%q): %v", raw, err)
	}
	return u
}

func TestDoGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Method", r.Method)
		io.WriteString(w, "hello from server")
	}))
	defer srv.Close()

	c := New(Config{Dial: plainDialer()})
	res, err := c.Do(mustURL(t, srv.URL))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.Response.StatusCode != 200 {
		t.Errorf("status = %d", res.Response.StatusCode)
	}
	if string(res.Response.Body) != "hello from server" {
		t.Errorf("body = %q", res.Response.Body)
	}
	if m, _ := res.Response.Headers.Get("X-Method"); m != "GET" {
		t.Errorf("server saw method %q, want GET", m)
	}
}

func TestDoCustomMethod(t *testing.T) {
	var seenMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(Config{Method: "DELETE", Dial: plainDialer()})
	if _, err := c.Do(mustURL(t, srv.URL)); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if seenMethod != "DELETE" {
		t.Errorf("server saw method %q, want DELETE", seenMethod)
	}
}

func TestDoPOSTBody(t *testing.T) {
	var seenBody, seenCT string
	var seenMethod string
	var seenCL int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenCT = r.Header.Get("Content-Type")
		seenCL = r.ContentLength
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		w.WriteHeader(201)
	}))
	defer srv.Close()

	c := New(Config{Body: []byte("name=fred"), Dial: plainDialer()})
	res, err := c.Do(mustURL(t, srv.URL))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if seenMethod != "POST" {
		t.Errorf("method = %q, want POST (body implies POST)", seenMethod)
	}
	if seenBody != "name=fred" {
		t.Errorf("body = %q", seenBody)
	}
	if seenCL != int64(len("name=fred")) {
		t.Errorf("Content-Length = %d, want %d", seenCL, len("name=fred"))
	}
	if seenCT != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", seenCT)
	}
	if res.Response.StatusCode != 201 {
		t.Errorf("status = %d", res.Response.StatusCode)
	}
}

func TestDoCustomHeader(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Custom")
	}))
	defer srv.Close()

	c := New(Config{
		Headers: []httpmsg.Header{{Name: "X-Custom", Value: "from-cc-curl"}},
		Dial:    plainDialer(),
	})
	if _, err := c.Do(mustURL(t, srv.URL)); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if seen != "from-cc-curl" {
		t.Errorf("server saw X-Custom = %q", seen)
	}
}

func TestDoChunkedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter is not a Flusher")
		}
		// Writing without Content-Length and flushing => chunked encoding.
		io.WriteString(w, "chunk-one;")
		fl.Flush()
		io.WriteString(w, "chunk-two;")
		fl.Flush()
		io.WriteString(w, "chunk-three")
		fl.Flush()
	}))
	defer srv.Close()

	c := New(Config{Dial: plainDialer()})
	res, err := c.Do(mustURL(t, srv.URL))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if te, _ := res.Response.Headers.Get("Transfer-Encoding"); te != "chunked" {
		t.Errorf("Transfer-Encoding = %q, want chunked", te)
	}
	if got := string(res.Response.Body); got != "chunk-one;chunk-two;chunk-three" {
		t.Errorf("body = %q", got)
	}
}

func TestDoFollowRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			io.WriteString(w, "arrived")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Without -L: see the 302.
	c := New(Config{Dial: plainDialer()})
	res, err := c.Do(mustURL(t, srv.URL+"/start"))
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.Response.StatusCode != 302 {
		t.Errorf("without -L status = %d, want 302", res.Response.StatusCode)
	}

	// With -L: follow to /final.
	c2 := New(Config{FollowRedir: true, Dial: plainDialer()})
	res2, err := c2.Do(mustURL(t, srv.URL+"/start"))
	if err != nil {
		t.Fatalf("Do (follow): %v", err)
	}
	if res2.Response.StatusCode != 200 {
		t.Errorf("with -L status = %d, want 200", res2.Response.StatusCode)
	}
	if string(res2.Response.Body) != "arrived" {
		t.Errorf("with -L body = %q, want arrived", res2.Response.Body)
	}
	if !strings.HasSuffix(res2.FinalURL.Path, "/final") {
		t.Errorf("final path = %q", res2.FinalURL.Path)
	}
}

func TestDoRedirectLoopGuard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer srv.Close()

	c := New(Config{FollowRedir: true, MaxRedirs: 3, Dial: plainDialer()})
	_, err := c.Do(mustURL(t, srv.URL+"/loop"))
	if err == nil {
		t.Fatal("expected max-redirects error")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Errorf("error = %v", err)
	}
}

func TestDoPOSTRedirectBecomesGET(t *testing.T) {
	var finalMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/post":
			http.Redirect(w, r, "/after", http.StatusSeeOther) // 303
		case "/after":
			finalMethod = r.Method
			io.WriteString(w, "ok")
		}
	}))
	defer srv.Close()

	c := New(Config{Body: []byte("x=1"), FollowRedir: true, Dial: plainDialer()})
	if _, err := c.Do(mustURL(t, srv.URL+"/post")); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if finalMethod != "GET" {
		t.Errorf("after 303 method = %q, want GET", finalMethod)
	}
}

func TestDoVerboseTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "body")
	}))
	defer srv.Close()

	var buf strings.Builder
	c := New(Config{Tracer: trace.New(&buf), Dial: plainDialer()})
	if _, err := c.Do(mustURL(t, srv.URL)); err != nil {
		t.Fatalf("Do: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "> GET / HTTP/1.1") {
		t.Errorf("trace missing sent request line:\n%s", out)
	}
	if !strings.Contains(out, "> Host: ") {
		t.Errorf("trace missing sent Host header:\n%s", out)
	}
	if !strings.Contains(out, "< HTTP/1.1 200") {
		t.Errorf("trace missing received status line:\n%s", out)
	}
}

func TestDoHTTPS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "secure hello")
	}))
	defer srv.Close()

	c := New(Config{Dial: tlsDialer()})
	res, err := c.Do(mustURL(t, srv.URL))
	if err != nil {
		t.Fatalf("Do (https): %v", err)
	}
	if string(res.Response.Body) != "secure hello" {
		t.Errorf("body = %q", res.Response.Body)
	}
}

func TestDoHEAD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "12")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(Config{Method: "HEAD", Dial: plainDialer()})
	res, err := c.Do(mustURL(t, srv.URL))
	if err != nil {
		t.Fatalf("Do (HEAD): %v", err)
	}
	if res.Response.Body != nil {
		t.Errorf("HEAD body = %q, want nil", res.Response.Body)
	}
}
