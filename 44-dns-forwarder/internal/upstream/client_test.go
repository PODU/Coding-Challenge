package upstream

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// fakeUDP spins up an in-process UDP server. The handler receives each query
// and returns the bytes to send back (or nil to stay silent). It returns the
// server address and a stop function.
func fakeUDP(t *testing.T, handler func(query []byte) []byte) (string, func()) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 65535)
		for {
			pc.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			n, addr, err := pc.ReadFrom(buf)
			select {
			case <-done:
				return
			default:
			}
			if err != nil {
				continue
			}
			q := append([]byte(nil), buf[:n]...)
			if resp := handler(q); resp != nil {
				pc.WriteTo(resp, addr)
			}
		}
	}()
	return pc.LocalAddr().String(), func() {
		close(done)
		pc.Close()
	}
}

func aQuery(id uint16) []byte {
	b := make([]byte, 12)
	binary.BigEndian.PutUint16(b[0:2], id)
	binary.BigEndian.PutUint16(b[2:4], 0x0100)
	binary.BigEndian.PutUint16(b[4:6], 1)
	b = append(b, 3, 'w', 'w', 'w', 6, 'g', 'o', 'o', 'g', 'l', 'e', 3, 'c', 'o', 'm', 0, 0, 1, 0, 1)
	return b
}

func TestExchangeSuccess(t *testing.T) {
	canned := []byte{0xDE, 0xAD, 0x81, 0x80, 0, 1, 0, 0, 0, 0, 0, 0, 't', 'e', 's', 't'}
	addr, stop := fakeUDP(t, func(q []byte) []byte {
		resp := append([]byte(nil), canned...)
		// Echo the query ID so the client accepts the response.
		copy(resp[0:2], q[0:2])
		return resp
	})
	defer stop()

	c := New(addr, time.Second)
	resp, err := c.Exchange(context.Background(), aQuery(0x1234))
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if binary.BigEndian.Uint16(resp[0:2]) != 0x1234 {
		t.Errorf("response ID = %#x, want 0x1234", binary.BigEndian.Uint16(resp[0:2]))
	}
	if string(resp[12:]) != "test" {
		t.Errorf("body = %q, want test", resp[12:])
	}
}

func TestExchangeTimeout(t *testing.T) {
	addr, stop := fakeUDP(t, func(q []byte) []byte { return nil }) // never replies
	defer stop()

	c := New(addr, 100*time.Millisecond)
	c.Retries = 0
	start := time.Now()
	if _, err := c.Exchange(context.Background(), aQuery(1)); err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestExchangeIgnoresWrongID(t *testing.T) {
	first := true
	addr, stop := fakeUDP(t, func(q []byte) []byte {
		resp := make([]byte, 12)
		copy(resp, q[:12])
		if first {
			first = false
			binary.BigEndian.PutUint16(resp[0:2], 0xFFFF) // wrong ID first
		}
		// Subsequent reads echo correct ID. Send the wrong one; the client
		// will keep reading until the deadline, but our server only replies on
		// receipt. So instead send both: wrong then the real one.
		return resp
	})
	defer stop()

	// Because the fake only answers per-datagram, simulate the stray by having
	// the client tolerate it: send query, server replies wrong ID, client keeps
	// waiting -> times out. We assert it does NOT accept the wrong ID.
	c := New(addr, 200*time.Millisecond)
	c.Retries = 0
	_, err := c.Exchange(context.Background(), aQuery(0x2222))
	if err == nil {
		t.Fatal("expected timeout: wrong-ID reply must be ignored")
	}
}
