package client

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/codingchallenges/ccmc/internal/protocol"
)

func dialFake(t *testing.T, s *fakeServer) *Client {
	t.Helper()
	host, port := s.hostPort()
	c, err := Dial(host, port, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestSetThenGet(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	status, err := c.Store(protocol.StorageCommand{Command: "set", Key: "testkey", Data: []byte("testvalue")})
	if err != nil {
		t.Fatal(err)
	}
	if status != protocol.Stored {
		t.Fatalf("set: got %v, want STORED", status)
	}

	vals, err := c.Get([]string{"testkey"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 1 || string(vals[0].Data) != "testvalue" {
		t.Fatalf("get round-trip failed: %+v", vals)
	}
}

func TestGetMiss(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	vals, err := c.Get([]string{"absent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 0 {
		t.Fatalf("expected miss, got %+v", vals)
	}
}

func TestAddTwice(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	if st, err := c.Store(protocol.StorageCommand{Command: "add", Key: "k", Data: []byte("v1")}); err != nil || st != protocol.Stored {
		t.Fatalf("first add: st=%v err=%v", st, err)
	}
	if st, err := c.Store(protocol.StorageCommand{Command: "add", Key: "k", Data: []byte("v2")}); err != nil || st != protocol.NotStored {
		t.Fatalf("second add: got %v, want NOT_STORED (err=%v)", st, err)
	}
}

func TestReplaceMissing(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	if st, err := c.Store(protocol.StorageCommand{Command: "replace", Key: "ghost", Data: []byte("v")}); err != nil || st != protocol.NotStored {
		t.Fatalf("replace missing: got %v, want NOT_STORED (err=%v)", st, err)
	}
}

func TestAppendPrepend(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	c.Store(protocol.StorageCommand{Command: "set", Key: "k", Data: []byte("middle")})
	if st, err := c.Store(protocol.StorageCommand{Command: "append", Key: "k", Data: []byte("-end")}); err != nil || st != protocol.Stored {
		t.Fatalf("append: %v %v", st, err)
	}
	if st, err := c.Store(protocol.StorageCommand{Command: "prepend", Key: "k", Data: []byte("start-")}); err != nil || st != protocol.Stored {
		t.Fatalf("prepend: %v %v", st, err)
	}
	vals, _ := c.Get([]string{"k"})
	if len(vals) != 1 || string(vals[0].Data) != "start-middle-end" {
		t.Fatalf("append/prepend result: %+v", vals)
	}

	// append on a missing key -> NOT_STORED
	if st, _ := c.Store(protocol.StorageCommand{Command: "append", Key: "nope", Data: []byte("x")}); st != protocol.NotStored {
		t.Fatalf("append missing: got %v, want NOT_STORED", st)
	}
}

func TestMultiKeyGetMixed(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	c.Store(protocol.StorageCommand{Command: "set", Key: "a", Data: []byte("AA")})
	c.Store(protocol.StorageCommand{Command: "set", Key: "c", Data: []byte("CC")})

	vals, err := c.Get([]string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, v := range vals {
		got[v.Key] = string(v.Data)
	}
	if len(got) != 2 || got["a"] != "AA" || got["c"] != "CC" {
		t.Fatalf("multi-key get mixed: %+v", got)
	}
	if _, ok := got["b"]; ok {
		t.Fatal("b should be a miss")
	}
}

func TestDeleteThenGet(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	c.Store(protocol.StorageCommand{Command: "set", Key: "k", Data: []byte("v")})
	if ok, err := c.Delete("k", false); err != nil || !ok {
		t.Fatalf("delete: ok=%v err=%v", ok, err)
	}
	vals, _ := c.Get([]string{"k"})
	if len(vals) != 0 {
		t.Fatalf("expected miss after delete, got %+v", vals)
	}
	if ok, err := c.Delete("k", false); err != nil || ok {
		t.Fatalf("delete missing: expected NOT_FOUND, got ok=%v err=%v", ok, err)
	}
}

func TestIncrDecr(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	c.Store(protocol.StorageCommand{Command: "set", Key: "n", Data: []byte("10")})
	if v, found, err := c.IncrDecr("incr", "n", 5, false); err != nil || !found || v != 15 {
		t.Fatalf("incr: v=%d found=%v err=%v", v, found, err)
	}
	if v, found, err := c.IncrDecr("decr", "n", 3, false); err != nil || !found || v != 12 {
		t.Fatalf("decr: v=%d found=%v err=%v", v, found, err)
	}
	// not found
	if _, found, err := c.IncrDecr("incr", "missing", 1, false); err != nil || found {
		t.Fatalf("incr missing: found=%v err=%v", found, err)
	}
	// non-numeric -> CLIENT_ERROR
	c.Store(protocol.StorageCommand{Command: "set", Key: "word", Data: []byte("hello")})
	_, _, err := c.IncrDecr("incr", "word", 1, false)
	var perr *protocol.ProtocolError
	if !errors.As(err, &perr) || !perr.IsClientError() {
		t.Fatalf("expected CLIENT_ERROR for non-numeric incr, got %v", err)
	}
}

func TestCAS(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	c.Store(protocol.StorageCommand{Command: "set", Key: "k", Data: []byte("v")})
	vals, _ := c.Gets([]string{"k"})
	if len(vals) != 1 || !vals[0].HasCAS {
		t.Fatalf("gets did not return CAS: %+v", vals)
	}
	casID := vals[0].CAS

	// stale cas -> EXISTS
	if st, err := c.Store(protocol.StorageCommand{Command: "cas", Key: "k", Data: []byte("v2"), CAS: casID + 999, HasCAS: true}); err != nil || st != protocol.Exists {
		t.Fatalf("stale cas: got %v, want EXISTS (err=%v)", st, err)
	}
	// correct cas -> STORED
	if st, err := c.Store(protocol.StorageCommand{Command: "cas", Key: "k", Data: []byte("v2"), CAS: casID, HasCAS: true}); err != nil || st != protocol.Stored {
		t.Fatalf("good cas: got %v, want STORED (err=%v)", st, err)
	}
	// cas on missing key -> NOT_FOUND
	if st, _ := c.Store(protocol.StorageCommand{Command: "cas", Key: "ghost", Data: []byte("x"), CAS: 1, HasCAS: true}); st != protocol.NotFound {
		t.Fatalf("cas missing: got %v, want NOT_FOUND", st)
	}
}

func TestStats(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	c.Store(protocol.StorageCommand{Command: "set", Key: "k", Data: []byte("v")})
	stats, err := c.Stats("")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, st := range stats {
		found[st.Name] = st.Value
	}
	if found["pid"] != "4242" || found["curr_items"] != "1" {
		t.Fatalf("unexpected stats: %+v", found)
	}
}

func TestBinaryValueRoundTrip(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	data := []byte{0x00, '\r', '\n', 0xff, 'x', 0x00}
	c.Store(protocol.StorageCommand{Command: "set", Key: "bin", Data: data})
	vals, err := c.Get([]string{"bin"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 1 || string(vals[0].Data) != string(data) {
		t.Fatalf("binary round-trip failed: %+v", vals)
	}
}

func TestNoReply(t *testing.T) {
	s := newFakeServer(t)
	c := dialFake(t, s)

	st, err := c.Store(protocol.StorageCommand{Command: "set", Key: "k", Data: []byte("v"), NoReply: true})
	if err != nil || st != protocol.NoReply {
		t.Fatalf("noreply set: st=%v err=%v", st, err)
	}
	// The value must still be retrievable, proving the server consumed the data.
	vals, err := c.Get([]string{"k"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 1 || string(vals[0].Data) != "v" {
		t.Fatalf("noreply set did not persist: %+v", vals)
	}
}

func TestConnectionRefused(t *testing.T) {
	// Listen then immediately close to obtain a definitely-unused port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	ln.Close()

	if _, err := Dial("127.0.0.1", port, 500*time.Millisecond); err == nil {
		t.Fatal("expected connection error to closed port")
	}
}

func TestReadTimeout(t *testing.T) {
	// A listener that accepts but never replies should trip the read deadline.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Hold the connection open without responding.
		defer conn.Close()
		buf := make([]byte, 256)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c, err := Dial(host, port, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	start := time.Now()
	_, err = c.Get([]string{"k"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("timeout took too long: %v", time.Since(start))
	}
}
