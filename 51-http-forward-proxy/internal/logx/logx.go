// Package logx is the proxy's access log. Lines go to stdout and a log file.
// log.Logger serializes writes, so a Logger is safe to share.
package logx

import (
	"io"
	"log"
	"os"
)

type Logger struct {
	l       *log.Logger
	closers []io.Closer
}

// New returns a Logger that writes to stdout and to each of extra.
func New(extra ...io.Writer) *Logger {
	writers := make([]io.Writer, 0, len(extra)+1)
	writers = append(writers, os.Stdout)
	writers = append(writers, extra...)
	return &Logger{
		l: log.New(io.MultiWriter(writers...), "", log.LstdFlags|log.LUTC),
	}
}

// NewWithFile returns a Logger that writes to stdout and appends to path.
// Call Close when done.
func NewWithFile(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	lg := New(f)
	lg.closers = append(lg.closers, f)
	return lg, nil
}

// Access logs one proxied request and the status sent to the client.
func (lg *Logger) Access(clientIP, url string, status int) {
	lg.l.Printf("ACCESS client=%s url=%q status=%d", clientIP, url, status)
}

// Tunnel logs a CONNECT tunnel event ("open" or "close").
func (lg *Logger) Tunnel(clientIP, target, event string) {
	lg.l.Printf("CONNECT client=%s target=%q event=%s", clientIP, target, event)
}

func (lg *Logger) Infof(format string, args ...any) {
	lg.l.Printf("INFO "+format, args...)
}

func (lg *Logger) Close() error {
	var first error
	for _, c := range lg.closers {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
