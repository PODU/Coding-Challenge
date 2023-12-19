// Package transport opens a TCP connection for http or a TLS one for https.
package transport

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/codingchallenges/cc-curl/internal/trace"
	"github.com/codingchallenges/cc-curl/internal/urlparse"
)

type Config struct {
	DialTimeout time.Duration
	Insecure    bool // skip certificate verification (-k)
	Tracer      *trace.Tracer
}

// DefaultDialTimeout applies when Config.DialTimeout is zero.
const DefaultDialTimeout = 30 * time.Second

// Dial connects to u's host, tracing the connection details in verbose mode.
func Dial(u urlparse.URL, cfg Config) (net.Conn, error) {
	timeout := cfg.DialTimeout
	if timeout == 0 {
		timeout = DefaultDialTimeout
	}
	addr := u.DialAddress()

	if u.Scheme == "https" {
		return dialTLS(u, addr, timeout, cfg)
	}

	cfg.Tracer.Info(fmt.Sprintf("Trying %s...", addr))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	cfg.Tracer.Info(fmt.Sprintf("Connected to %s (%s)", u.Host, conn.RemoteAddr()))
	return conn, nil
}

func dialTLS(u urlparse.URL, addr string, timeout time.Duration, cfg Config) (net.Conn, error) {
	cfg.Tracer.Info(fmt.Sprintf("Trying %s...", addr))
	dialer := &net.Dialer{Timeout: timeout}
	tlsCfg := &tls.Config{
		ServerName:         u.Host,
		InsecureSkipVerify: cfg.Insecure,
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("TLS dial %s: %w", addr, err)
	}
	if cfg.Tracer.Enabled() {
		state := conn.ConnectionState()
		cfg.Tracer.Info(fmt.Sprintf("Connected to %s (%s)", u.Host, conn.RemoteAddr()))
		cfg.Tracer.Info(fmt.Sprintf("SSL connection using %s / %s",
			tlsVersionName(state.Version), tls.CipherSuiteName(state.CipherSuite)))
		if len(state.PeerCertificates) > 0 {
			cfg.Tracer.Info(fmt.Sprintf("server certificate: subject=%q",
				state.PeerCertificates[0].Subject.String()))
		}
	}
	return conn, nil
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLSv1.3"
	case tls.VersionTLS12:
		return "TLSv1.2"
	case tls.VersionTLS11:
		return "TLSv1.1"
	case tls.VersionTLS10:
		return "TLSv1.0"
	default:
		return "TLS"
	}
}
