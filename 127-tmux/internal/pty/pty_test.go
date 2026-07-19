package pty

import (
	"io"
	"testing"
)

func TestMockEchoesWrites(t *testing.T) {
	m := NewMock(80, 24, true)
	m.Write([]byte("hi\r"))
	buf := make([]byte, 16)
	n, err := m.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	// \r is expanded to \r\n like a tty in echo mode.
	if string(buf[:n]) != "hi\r\n" {
		t.Fatalf("echo = %q", buf[:n])
	}
}

func TestMockFeedAndResize(t *testing.T) {
	m := NewMock(80, 24, false)
	m.Feed([]byte("boot"))
	buf := make([]byte, 8)
	n, _ := m.Read(buf)
	if string(buf[:n]) != "boot" {
		t.Fatalf("feed = %q", buf[:n])
	}
	if err := m.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	if w, h := m.Size(); w != 100 || h != 30 {
		t.Fatalf("size = %dx%d", w, h)
	}
	if m.Resizes() != 1 {
		t.Fatalf("resizes = %d", m.Resizes())
	}
}

func TestMockCloseReturnsEOF(t *testing.T) {
	m := NewMock(80, 24, false)
	m.Close()
	buf := make([]byte, 4)
	if _, err := m.Read(buf); err != io.EOF {
		t.Fatalf("closed read err = %v, want EOF", err)
	}
	if _, err := m.Write([]byte("x")); err != ErrClosed {
		t.Fatalf("closed write err = %v, want ErrClosed", err)
	}
}

func TestMockNonBlockDrain(t *testing.T) {
	m := NewMock(10, 5, false)
	if m.Available() != 0 {
		t.Fatal("fresh mock should have nothing available")
	}
	m.Feed([]byte("abc"))
	if m.Available() != 3 {
		t.Fatalf("available = %d", m.Available())
	}
	buf := make([]byte, 2)
	n, _ := m.ReadNonBlock(buf)
	if n != 2 || string(buf[:n]) != "ab" {
		t.Fatalf("nonblock read = %q", buf[:n])
	}
	if m.Available() != 1 {
		t.Fatalf("remaining = %d", m.Available())
	}
}
