package protocol

import (
	"bytes"
	"io"
	"testing"
)

func TestRoundTripSingle(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMessage(&buf, NewSession, []byte("work")); err != nil {
		t.Fatal(err)
	}
	m, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != NewSession || m.Str() != "work" {
		t.Fatalf("got %v %q", m.Type, m.Str())
	}
}

func TestRoundTripStream(t *testing.T) {
	var buf bytes.Buffer
	msgs := []Message{
		{AttachSession, []byte("main")},
		{Input, []byte("ls\r")},
		{Output, []byte("\x1b[2Jhello")},
		{Detach, nil},
	}
	for _, m := range msgs {
		if err := WriteMessage(&buf, m.Type, m.Payload); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range msgs {
		got, err := ReadMessage(&buf)
		if err != nil {
			t.Fatalf("msg %d: %v", i, err)
		}
		if got.Type != want.Type || !bytes.Equal(got.Payload, want.Payload) {
			t.Fatalf("msg %d: got %v %q want %v %q", i, got.Type, got.Payload, want.Type, want.Payload)
		}
	}
	if _, err := ReadMessage(&buf); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestEmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	WriteMessage(&buf, Detach, nil)
	m, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != Detach || len(m.Payload) != 0 {
		t.Fatalf("got %v len %d", m.Type, len(m.Payload))
	}
}

func TestResizeCodec(t *testing.T) {
	p := EncodeResize(120, 40)
	w, h, err := DecodeResize(p)
	if err != nil || w != 120 || h != 40 {
		t.Fatalf("w=%d h=%d err=%v", w, h, err)
	}
	if _, _, err := DecodeResize([]byte("garbage")); err == nil {
		t.Fatal("expected error on bad payload")
	}
}

func TestTruncatedStreamIsEOF(t *testing.T) {
	// A header promising more bytes than present should not panic.
	buf := bytes.NewReader([]byte{0, 0, 0, 10, byte(Input), 'a'})
	if _, err := ReadMessage(buf); err == nil {
		t.Fatal("expected error on truncated frame")
	}
}
