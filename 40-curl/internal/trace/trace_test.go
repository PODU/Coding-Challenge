package trace

import (
	"strings"
	"testing"
)

func TestTracerPrefixes(t *testing.T) {
	var buf strings.Builder
	tr := New(&buf)
	tr.Info("connected")
	tr.Sent("GET / HTTP/1.1")
	tr.Recv("HTTP/1.1 200 OK")

	want := "* connected\n> GET / HTTP/1.1\n< HTTP/1.1 200 OK\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestTracerNilIsNoop(t *testing.T) {
	var tr *Tracer
	if tr.Enabled() {
		t.Error("nil tracer should be disabled")
	}
	// Should not panic.
	tr.Info("x")
	tr.Sent("y")
	tr.Recv("z")
	tr.SentBlock("a\r\nb")
}

func TestTracerSentBlock(t *testing.T) {
	var buf strings.Builder
	tr := New(&buf)
	tr.SentBlock("GET / HTTP/1.1\r\nHost: x\r\nAccept: */*")
	want := "> GET / HTTP/1.1\n> Host: x\n> Accept: */*\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}
