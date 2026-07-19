// Package pty runs a shell on a pseudo-terminal. The real implementation is
// Linux-only; Mock works everywhere and is used by the tests and the demo.
package pty

import (
	"errors"
	"io"
	"sync"
)

// PTY connects to a shell. Read blocks until there is output, and returns
// io.EOF once the PTY is closed.
type PTY interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(w, h int) error
	Close() error
}

var ErrClosed = errors.New("pty: closed")

// Mock is an in-memory PTY that can echo writes back like a terminal in echo
// mode. Feed injects output directly. Read blocks like a real PTY would.
type Mock struct {
	mu      sync.Mutex
	cond    *sync.Cond
	buf     []byte
	closed  bool
	echo    bool
	w, h    int
	resizes int
}

// NewMock returns a Mock. With echo set, writes are echoed back with CR turned
// into CR LF, as a tty does.
func NewMock(w, h int, echo bool) *Mock {
	m := &Mock{echo: echo, w: w, h: h}
	m.cond = sync.NewCond(&m.mu)
	return m
}

// Feed queues p as if the shell had printed it.
func (m *Mock) Feed(p []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf = append(m.buf, p...)
	m.cond.Broadcast()
}

func (m *Mock) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for len(m.buf) == 0 && !m.closed {
		m.cond.Wait()
	}
	if len(m.buf) == 0 && m.closed {
		return 0, io.EOF
	}
	n := copy(p, m.buf)
	m.buf = m.buf[n:]
	return n, nil
}

// Available reports how many bytes are waiting to be read.
func (m *Mock) Available() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.buf)
}

// ReadNonBlock is Read without the wait: it returns 0 if nothing is buffered.
func (m *Mock) ReadNonBlock(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.buf) == 0 {
		return 0, nil
	}
	n := copy(p, m.buf)
	m.buf = m.buf[n:]
	return n, nil
}

func (m *Mock) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, ErrClosed
	}
	if m.echo {
		for _, b := range p {
			if b == '\r' {
				m.buf = append(m.buf, '\r', '\n')
			} else {
				m.buf = append(m.buf, b)
			}
		}
		m.cond.Broadcast()
	}
	return len(p), nil
}

func (m *Mock) Resize(w, h int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.w, m.h = w, h
	m.resizes++
	return nil
}

func (m *Mock) Size() (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.w, m.h
}

// Resizes counts the calls to Resize.
func (m *Mock) Resizes() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resizes
}

func (m *Mock) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	m.cond.Broadcast()
	return nil
}
