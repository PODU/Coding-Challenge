package proxy

import (
	"net"
	"net/http"
	"time"
)

// NewTransport builds the transport shared by all backends. The dial and
// response-header timeouts stop a hung backend from tying up a goroutine forever.
func NewTransport(dialTimeout, respHdrTimeout time.Duration) *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: respHdrTimeout,
	}
}
