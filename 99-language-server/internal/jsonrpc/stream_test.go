package jsonrpc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// Marshal a message and read it back; struct fields match.
func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	out := NewStream(nil, &buf)
	if err := out.Write(NewRequest(7, "initialize", map[string]int{"x": 1})); err != nil {
		t.Fatalf("write: %v", err)
	}

	in := NewStream(&buf, nil)
	got, err := in.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !got.IsRequest() {
		t.Fatalf("expected a request, got %+v", got)
	}
	if got.Method != "initialize" {
		t.Errorf("method = %q, want initialize", got.Method)
	}
	if string(got.ID) != "7" {
		t.Errorf("id = %s, want 7", got.ID)
	}
	if string(got.Params) != `{"x":1}` {
		t.Errorf("params = %s", got.Params)
	}
}

// A hand-written frame with a trailing Content-Type header parses fine,
// the extra header being ignored.
func TestDecodeWithExtraHeader(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	frame := fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n%s", len(body), body)

	msg, err := NewStream(strings.NewReader(frame), nil).Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if msg.Method != "ping" || string(msg.ID) != "1" {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

// A frame with a non-integer Content-Length is rejected.
func TestRejectGarbageContentLength(t *testing.T) {
	frame := "Content-Length: abc\r\n\r\n{}"
	_, err := NewStream(strings.NewReader(frame), nil).Read()
	var fe *FramingError
	if !errors.As(err, &fe) {
		t.Fatalf("want *FramingError, got %v", err)
	}
}

// A header block missing Content-Length entirely is rejected.
func TestRejectMissingContentLength(t *testing.T) {
	frame := "Content-Type: x\r\n\r\n{}"
	_, err := NewStream(strings.NewReader(frame), nil).Read()
	var fe *FramingError
	if !errors.As(err, &fe) {
		t.Fatalf("want *FramingError, got %v", err)
	}
}

// Two messages back-to-back decode as two distinct messages (no over-read).
func TestTwoMessagesBackToBack(t *testing.T) {
	var buf bytes.Buffer
	out := NewStream(nil, &buf)
	_ = out.Write(NewNotification("a", nil))
	_ = out.Write(NewNotification("b", nil))

	in := NewStream(&buf, nil)
	m1, err := in.Read()
	if err != nil {
		t.Fatalf("read 1: %v", err)
	}
	m2, err := in.Read()
	if err != nil {
		t.Fatalf("read 2: %v", err)
	}
	if m1.Method != "a" || m2.Method != "b" {
		t.Fatalf("got %q, %q; want a, b", m1.Method, m2.Method)
	}
	if _, err := in.Read(); !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF after two messages, got %v", err)
	}
}

// A multibyte UTF-8 body is read by exact byte count.
func TestMultibyteBody(t *testing.T) {
	var buf bytes.Buffer
	out := NewStream(nil, &buf)
	want := "café 日本語" // mixed 2- and 3-byte runes
	_ = out.Write(NewNotification("greet", map[string]string{"text": want}))

	msg, err := NewStream(&buf, nil).Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(msg.Params), want) {
		t.Fatalf("multibyte body mangled: %s", msg.Params)
	}
}

// A Content-Length over the cap is rejected without reading the body.
func TestContentLengthCap(t *testing.T) {
	s := NewStream(strings.NewReader("Content-Length: 100\r\n\r\n"), nil)
	s.MaxBody = 10
	_, err := s.Read()
	var fe *FramingError
	if !errors.As(err, &fe) {
		t.Fatalf("want *FramingError for oversized body, got %v", err)
	}
}

// EOF in the middle of a body is reported as ErrUnexpectedEOF.
func TestTruncatedBody(t *testing.T) {
	frame := "Content-Length: 50\r\n\r\n{\"jsonrpc\":\"2.0\"}" // body shorter than 50
	_, err := NewStream(strings.NewReader(frame), nil).Read()
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("want ErrUnexpectedEOF, got %v", err)
	}
}

// An invalid JSON body yields a *ParseError carrying the recovered id.
func TestParseErrorRecoversID(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":42,"method":}`
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	_, err := NewStream(strings.NewReader(frame), nil).Read()
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ParseError, got %v", err)
	}
	if string(pe.ID) != "42" {
		t.Fatalf("recovered id = %s, want 42", pe.ID)
	}
}
