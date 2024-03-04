// Package proxy is the HTTP forward proxy handler. It forwards absolute-form
// requests, tunnels CONNECT for HTTPS, and applies the host and banned-word
// filters.
package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/podu2997/http-forward-proxy/internal/filter"
	"github.com/podu2997/http-forward-proxy/internal/logx"
)

// hopByHopHeaders apply to a single connection and must not be forwarded in
// either direction (RFC 7230 section 6.1).
var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// maxScanBody limits how much of a response we buffer for the banned-word
// scan. Larger bodies are blocked rather than passed through unscanned.
const maxScanBody = 8 << 20 // 8 MiB

type Handler struct {
	transport *http.Transport
	filter    *filter.Filter
	logger    *logx.Logger
	dialer    *net.Dialer
	verbose   bool
}

type Config struct {
	Filter      *filter.Filter
	Logger      *logx.Logger
	DialTimeout time.Duration
	IdleTimeout time.Duration
	Verbose     bool
}

// New returns a Handler. Zero fields in cfg get defaults.
func New(cfg Config) *Handler {
	if cfg.Filter == nil {
		cfg.Filter = filter.New()
	}
	if cfg.Logger == nil {
		cfg.Logger = logx.New()
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 90 * time.Second
	}
	dialer := &net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil, // don't chain to another proxy
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false, // stay on HTTP/1.1 so responses relay as-is
		MaxIdleConns:          100,
		IdleConnTimeout:       cfg.IdleTimeout,
		TLSHandshakeTimeout:   cfg.DialTimeout,
		ExpectContinueTimeout: time.Second,
	}
	return &Handler{
		transport: transport,
		filter:    cfg.Filter,
		logger:    cfg.Logger,
		dialer:    dialer,
		verbose:   cfg.Verbose,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		h.tunnel(w, r)
		return
	}
	h.forward(w, r)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) forward(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	// Requests to a proxy must carry the full URL (absolute-form).
	if !r.URL.IsAbs() || r.URL.Host == "" {
		http.Error(w, "400 Bad Request: proxy requires absolute-form request URI", http.StatusBadRequest)
		h.logger.Access(ip, r.RequestURI, http.StatusBadRequest)
		return
	}

	target := r.URL.String()

	if h.filter.HostForbidden(r.URL.Host) {
		h.deny(w, ip, target, "host is on the forbidden list")
		return
	}

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		h.logger.Access(ip, target, http.StatusBadRequest)
		return
	}
	copyHeaders(outReq.Header, r.Header)
	removeHopByHop(outReq.Header, r.Header)
	appendXForwardedFor(outReq.Header, ip)

	resp, err := h.transport.RoundTrip(outReq)
	if err != nil {
		if h.verbose {
			h.logger.Infof("roundtrip error for %s: %v", target, err)
		}
		http.Error(w, "502 Bad Gateway: "+err.Error(), http.StatusBadGateway)
		h.logger.Access(ip, target, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respHeader := w.Header()
	copyHeaders(respHeader, resp.Header)
	removeHopByHop(respHeader, resp.Header)

	// Only buffer the body when there is something to scan for.
	if h.filter.HasBannedWords() {
		h.forwardScanned(w, resp, ip, target)
		return
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	h.logger.Access(ip, target, resp.StatusCode)
}

// forwardScanned reads the whole body (up to maxScanBody) and checks it for
// banned words before sending anything to the client.
func (h *Handler) forwardScanned(w http.ResponseWriter, resp *http.Response, ip, target string) {
	limited := io.LimitReader(resp.Body, maxScanBody+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		h.logger.Access(ip, target, http.StatusBadGateway)
		return
	}
	if len(body) > maxScanBody {
		h.clearBodyHeaders(w.Header())
		h.deny(w, ip, target, "response exceeds scan limit")
		return
	}
	if word, hit := h.filter.BodyHasBanned(body); hit {
		h.clearBodyHeaders(w.Header())
		h.deny(w, ip, target, fmt.Sprintf("response contained banned content (%q)", word))
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
	h.logger.Access(ip, target, resp.StatusCode)
}

// clearBodyHeaders drops the origin's body headers before we replace the body
// with a 403 message.
func (h *Handler) clearBodyHeaders(hdr http.Header) {
	hdr.Del("Content-Length")
	hdr.Del("Content-Type")
	hdr.Del("Content-Encoding")
}

func (h *Handler) deny(w http.ResponseWriter, ip, target, reason string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, "403 Forbidden: %s\n", reason)
	h.logger.Access(ip, target, http.StatusForbidden)
}

// tunnel handles CONNECT: it dials the origin, hijacks the client connection,
// and relays bytes both ways until one side closes.
func (h *Handler) tunnel(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	authority := r.Host
	if authority == "" {
		authority = r.URL.Host
	}
	if authority == "" {
		http.Error(w, "400 Bad Request: empty CONNECT authority", http.StatusBadRequest)
		h.logger.Access(ip, "CONNECT", http.StatusBadRequest)
		return
	}

	host, port := splitHostPort(authority, "443")
	target := net.JoinHostPort(host, port)

	// Once the tunnel is up we only see TLS, so the host check has to happen now.
	if h.filter.HostForbidden(host) {
		h.deny(w, ip, target, "host is on the forbidden list")
		return
	}

	// Dial before hijacking so a failure can still be reported as plain HTTP.
	originConn, err := h.dialer.Dial("tcp", target)
	if err != nil {
		if h.verbose {
			h.logger.Infof("CONNECT dial error for %s: %v", target, err)
		}
		http.Error(w, "502 Bad Gateway: "+err.Error(), http.StatusBadGateway)
		h.logger.Access(ip, target, http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		originConn.Close()
		http.Error(w, "500 Internal Server Error: hijacking unsupported", http.StatusInternalServerError)
		h.logger.Access(ip, target, http.StatusInternalServerError)
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		originConn.Close()
		h.logger.Access(ip, target, http.StatusInternalServerError)
		return
	}

	// The ResponseWriter is off limits after Hijack.
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		clientConn.Close()
		originConn.Close()
		return
	}
	h.logger.Tunnel(ip, target, "open")
	h.logger.Access(ip, target, http.StatusOK)

	// The server may already have buffered client bytes, such as a pipelined
	// TLS ClientHello. Pass them on before relaying.
	if n := clientBuf.Reader.Buffered(); n > 0 {
		if pending, err := clientBuf.Reader.Peek(n); err == nil {
			originConn.Write(pending)
		}
	}

	pipe(clientConn, originConn)
	h.logger.Tunnel(ip, target, "close")
}

// pipe relays bytes between a and b until both directions are done. When one
// direction ends it closes both conns so the other io.Copy returns too.
func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	cp := func(dst, src net.Conn) {
		defer wg.Done()
		io.Copy(dst, src)
		dst.Close()
		src.Close()
	}
	go cp(a, b)
	go cp(b, a)
	wg.Wait()
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// removeHopByHop deletes the standard hop-by-hop headers from hdr, plus any
// header that src's Connection header lists.
func removeHopByHop(hdr, src http.Header) {
	for _, c := range src.Values("Connection") {
		for _, token := range strings.Split(c, ",") {
			if token = strings.TrimSpace(token); token != "" {
				hdr.Del(token)
			}
		}
	}
	for _, h := range hopByHopHeaders {
		hdr.Del(h)
	}
}

func appendXForwardedFor(hdr http.Header, ip string) {
	if ip == "" {
		return
	}
	if prior := hdr.Get("X-Forwarded-For"); prior != "" {
		hdr.Set("X-Forwarded-For", prior+", "+ip)
	} else {
		hdr.Set("X-Forwarded-For", ip)
	}
}

// splitHostPort is net.SplitHostPort with a default port.
func splitHostPort(authority, defPort string) (host, port string) {
	if h, p, err := net.SplitHostPort(authority); err == nil {
		return h, p
	}
	h := authority
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		h = h[1 : len(h)-1]
	}
	return h, defPort
}
