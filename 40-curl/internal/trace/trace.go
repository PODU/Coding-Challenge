// Package trace provides curl-style verbose output: lines prefixed with "> "
// for sent data, "< " for received data, and "* " for informational notes.
package trace

import (
	"bufio"
	"io"
	"strings"
)

// Tracer writes the -v output. A nil *Tracer does nothing, so callers don't need
// to check for it.
type Tracer struct {
	w io.Writer
}

// New returns nil (a disabled tracer) if w is nil.
func New(w io.Writer) *Tracer {
	if w == nil {
		return nil
	}
	return &Tracer{w: w}
}

// Enabled reports whether t writes anything.
func (t *Tracer) Enabled() bool {
	return t != nil && t.w != nil
}

func (t *Tracer) writePrefixed(prefix, line string) {
	if !t.Enabled() {
		return
	}
	io.WriteString(t.w, prefix)
	io.WriteString(t.w, line)
	io.WriteString(t.w, "\n")
}

// Info writes a "* " note.
func (t *Tracer) Info(line string) { t.writePrefixed("* ", line) }

// Sent writes a "> " line of request data.
func (t *Tracer) Sent(line string) { t.writePrefixed("> ", line) }

// Recv writes a "< " line of response data.
func (t *Tracer) Recv(line string) { t.writePrefixed("< ", line) }

// SentBlock writes each line of block with the "> " prefix.
func (t *Tracer) SentBlock(block string) {
	if !t.Enabled() {
		return
	}
	t.emitBlock("> ", block)
}

// RecvBlock writes each line of block with the "< " prefix.
func (t *Tracer) RecvBlock(block string) {
	if !t.Enabled() {
		return
	}
	t.emitBlock("< ", block)
}

func (t *Tracer) emitBlock(prefix, block string) {
	block = strings.ReplaceAll(block, "\r\n", "\n")
	block = strings.TrimSuffix(block, "\n")
	sc := bufio.NewScanner(strings.NewReader(block))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		t.writePrefixed(prefix, sc.Text())
	}
}
