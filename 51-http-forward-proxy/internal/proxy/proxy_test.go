package proxy

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/podu2997/http-forward-proxy/internal/filter"
	"github.com/podu2997/http-forward-proxy/internal/logx"
)

// writeTempList writes entries (one per line) to a temp file and returns its
// path, for seeding filter list-loaders.
func writeTempList(t *testing.T, entries ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(path, []byte(strings.Join(entries, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// newProxyServer wraps a Handler (built with the given filter and a
// buffer-backed logger) in an httptest.Server and returns the server plus the
// log buffer.
func newProxyServer(t *testing.T, f *filter.Filter) (*httptest.Server, *bytes.Buffer) {
	t.Helper()
	var logBuf bytes.Buffer
	h := New(Config{
		Filter:      f,
		Logger:      logx.New(&logBuf),
		DialTimeout: 5 * time.Second,
	})
	ps := httptest.NewServer(h)
	t.Cleanup(ps.Close)
	return ps, &logBuf
}

// clientThroughProxy returns an http.Client that sends all requests through
// the proxy at proxyURL.
func clientThroughProxy(t *testing.T, proxyURL string) *http.Client {
	t.Helper()
	pu, err := url.Parse(proxyURL)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(pu)},
	}
}

func TestForwardHappyPath(t *testing.T) {
	var gotXFF string
	var sawProxyConnection bool
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF = r.Header.Get("X-Forwarded-For")
		if r.Header.Get("Proxy-Connection") != "" {
			sawProxyConnection = true
		}
		w.Header().Set("X-Origin", "yes")
		io.WriteString(w, "hello from origin")
	}))
	defer origin.Close()

	ps, logBuf := newProxyServer(t, filter.New())
	client := clientThroughProxy(t, ps.URL)

	req, _ := http.NewRequest("GET", origin.URL+"/path", nil)
	req.Header.Set("Proxy-Connection", "keep-alive")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if string(body) != "hello from origin" {
		t.Errorf("body = %q", body)
	}
	if resp.Header.Get("X-Origin") != "yes" {
		t.Error("origin response header not relayed")
	}
	if gotXFF == "" {
		t.Error("origin did not see X-Forwarded-For")
	}
	if sawProxyConnection {
		t.Error("hop-by-hop Proxy-Connection header leaked to origin")
	}
	if !strings.Contains(logBuf.String(), "status=200") {
		t.Errorf("access log missing 200 line: %q", logBuf.String())
	}
}

func TestHopByHopConnectionTokenStripped(t *testing.T) {
	var sawCustom bool
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Hop") != "" {
			sawCustom = true
		}
		io.WriteString(w, "ok")
	}))
	defer origin.Close()

	ps, _ := newProxyServer(t, filter.New())
	client := clientThroughProxy(t, ps.URL)

	req, _ := http.NewRequest("GET", origin.URL, nil)
	// Name a custom header in Connection: it must be stripped per RFC 7230.
	req.Header.Set("Connection", "X-Custom-Hop")
	req.Header.Set("X-Custom-Hop", "should-be-removed")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if sawCustom {
		t.Error("connection-token-named header X-Custom-Hop was not stripped")
	}
}

func TestForwardHostBlocked(t *testing.T) {
	var originHit bool
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originHit = true
		io.WriteString(w, "should not be reached")
	}))
	defer origin.Close()

	ou, _ := url.Parse(origin.URL)
	f := filter.New()
	if _, err := f.LoadHosts(writeTempList(t, ou.Hostname())); err != nil {
		t.Fatal(err)
	}

	ps, logBuf := newProxyServer(t, f)
	client := clientThroughProxy(t, ps.URL)

	resp, err := client.Get(origin.URL + "/secret")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 403 {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if !strings.Contains(string(body), "403 Forbidden") {
		t.Errorf("body = %q, want forbidden message", body)
	}
	if originHit {
		t.Error("origin was contacted despite host block")
	}
	if !strings.Contains(logBuf.String(), "status=403") {
		t.Error("403 not logged")
	}
}

func TestBannedWordBlocking(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/bad") {
			io.WriteString(w, "this contains MALWARE definitely")
		} else {
			io.WriteString(w, "this is perfectly fine content")
		}
	}))
	defer origin.Close()

	f := filter.New()
	if _, err := f.LoadWords(writeTempList(t, "malware")); err != nil {
		t.Fatal(err)
	}

	ps, _ := newProxyServer(t, f)
	client := clientThroughProxy(t, ps.URL)

	// Banned body -> 403.
	resp, err := client.Get(origin.URL + "/bad")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("banned body status = %d, want 403", resp.StatusCode)
	}
	if strings.Contains(string(body), "MALWARE") {
		t.Error("banned content leaked to client")
	}

	// Clean body -> 200 passthrough.
	resp2, err := client.Get(origin.URL + "/good")
	if err != nil {
		t.Fatal(err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("clean body status = %d, want 200", resp2.StatusCode)
	}
	if string(body2) != "this is perfectly fine content" {
		t.Errorf("clean body altered: %q", body2)
	}
}

func TestForwardOriginDown(t *testing.T) {
	// An origin that is started then immediately closed yields a dead address.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := origin.URL
	origin.Close()

	ps, _ := newProxyServer(t, filter.New())
	client := clientThroughProxy(t, ps.URL)

	resp, err := client.Get(deadURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
}

func TestMalformedOriginFormRequest(t *testing.T) {
	ps, _ := newProxyServer(t, filter.New())

	// Hit the proxy directly with an origin-form request (not absolute-form),
	// i.e. a client that isn't actually using us as a proxy.
	resp, err := http.Get(ps.URL + "/just/a/path")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestConnectTunnel(t *testing.T) {
	// TLS origin reachable only by tunneling the TLS handshake end-to-end.
	tlsOrigin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "secure hello")
	}))
	defer tlsOrigin.Close()

	ps, logBuf := newProxyServer(t, filter.New())

	// Trust the test server's cert.
	certpool := x509.NewCertPool()
	certpool.AddCert(tlsOrigin.Certificate())
	pu, _ := url.Parse(ps.URL)
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(pu),
			TLSClientConfig: &tls.Config{RootCAs: certpool},
		},
	}

	resp, err := client.Get(tlsOrigin.URL) // https://... -> triggers CONNECT
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "secure hello" {
		t.Fatalf("tunneled response: status=%d body=%q", resp.StatusCode, body)
	}
	if !strings.Contains(logBuf.String(), "event=open") {
		t.Errorf("tunnel open not logged: %q", logBuf.String())
	}
}

func TestConnectBlockedHost(t *testing.T) {
	tlsOrigin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not reach")
	}))
	defer tlsOrigin.Close()

	ou, _ := url.Parse(tlsOrigin.URL)
	f := filter.New()
	if _, err := f.LoadHosts(writeTempList(t, ou.Hostname())); err != nil {
		t.Fatal(err)
	}

	ps, _ := newProxyServer(t, f)
	certpool := x509.NewCertPool()
	certpool.AddCert(tlsOrigin.Certificate())
	pu, _ := url.Parse(ps.URL)
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(pu),
			TLSClientConfig: &tls.Config{RootCAs: certpool},
		},
	}

	// The proxy returns 403 to the CONNECT, so the TLS round-trip must fail.
	_, err := client.Get(tlsOrigin.URL)
	if err == nil {
		t.Fatal("expected error for blocked CONNECT host, got nil")
	}
}

func TestConnectDeadOrigin(t *testing.T) {
	tlsOrigin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := tlsOrigin.URL
	tlsOrigin.Close()

	ps, _ := newProxyServer(t, filter.New())
	pu, _ := url.Parse(ps.URL)
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(pu),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// CONNECT to a dead origin: proxy writes 502, client's tunnel setup fails.
	_, err := client.Get(deadURL)
	if err == nil {
		t.Fatal("expected error for dead CONNECT origin, got nil")
	}
}

func TestSplitHostPort(t *testing.T) {
	cases := []struct {
		in       string
		wantHost string
		wantPort string
	}{
		{"example.com:443", "example.com", "443"},
		{"example.com", "example.com", "443"},
		{"[::1]:8443", "::1", "8443"},
		{"[::1]", "::1", "443"},
	}
	for _, c := range cases {
		h, p := splitHostPort(c.in, "443")
		if h != c.wantHost || p != c.wantPort {
			t.Errorf("splitHostPort(%q) = (%q,%q), want (%q,%q)", c.in, h, p, c.wantHost, c.wantPort)
		}
	}
}
