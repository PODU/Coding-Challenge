// Package backend models one upstream server. Health state and the in-flight
// counter are atomics so the request path never takes a lock.
package backend

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

// Backend represents one upstream server.
type Backend struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy

	alive       atomic.Bool
	activeConns atomic.Int64
}

// New builds a Backend for u. onError runs when the proxy cannot reach the
// upstream, so the caller can mark the backend down and answer the client.
// Backends start out alive; the health checker corrects that within one
// interval if they are not.
func New(u *url.URL, transport http.RoundTripper, onError func(b *Backend, w http.ResponseWriter, r *http.Request, err error)) *Backend {
	b := &Backend{URL: u}

	proxy := httputil.NewSingleHostReverseProxy(u)
	if transport != nil {
		proxy.Transport = transport
	}

	// Let the upstream see the host the client originally asked for.
	defaultDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		defaultDirector(r)
		if r.Header.Get("X-Forwarded-Host") == "" {
			r.Header.Set("X-Forwarded-Host", r.Host)
		}
	}

	if onError != nil {
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			onError(b, w, r, err)
		}
	}

	b.Proxy = proxy
	b.alive.Store(true)
	return b
}

// Alive reports whether the backend is currently considered healthy.
func (b *Backend) Alive() bool { return b.alive.Load() }

// SetAlive sets the backend's health state.
func (b *Backend) SetAlive(v bool) { b.alive.Store(v) }

// IncConns increments and returns the in-flight request counter.
func (b *Backend) IncConns() int64 { return b.activeConns.Add(1) }

// DecConns decrements and returns the in-flight request counter.
func (b *Backend) DecConns() int64 { return b.activeConns.Add(-1) }

// ActiveConns returns the current number of in-flight requests.
func (b *Backend) ActiveConns() int64 { return b.activeConns.Load() }
