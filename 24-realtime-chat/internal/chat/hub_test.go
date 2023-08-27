package chat

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

const readTimeout = 2 * time.Second

// startHub spins up a hub and returns it with a cleanup that shuts it down.
func startHub(t *testing.T) *Hub {
	t.Helper()
	h := NewHub()
	go h.Run()
	t.Cleanup(func() { h.Shutdown() })
	return h
}

// recv reads the next frame from a client, failing on timeout.
func recv(t *testing.T, c *Client) Outgoing {
	t.Helper()
	select {
	case data, ok := <-c.send:
		if !ok {
			t.Fatalf("send channel closed unexpectedly")
		}
		var out Outgoing
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatalf("bad frame: %v", err)
		}
		return out
	case <-time.After(readTimeout):
		t.Fatalf("timed out waiting for frame")
		return Outgoing{}
	}
}

// recvType reads frames until one of the wanted type arrives.
func recvType(t *testing.T, c *Client, typ string) Outgoing {
	t.Helper()
	deadline := time.After(readTimeout)
	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				t.Fatalf("send channel closed while waiting for %q", typ)
			}
			var out Outgoing
			if err := json.Unmarshal(data, &out); err == nil && out.Type == typ {
				return out
			}
		case <-deadline:
			t.Fatalf("timed out waiting for frame %q", typ)
		}
	}
}

// expectSilence asserts no frame arrives within d.
func expectSilence(t *testing.T, c *Client, d time.Duration) {
	t.Helper()
	select {
	case data, ok := <-c.send:
		if ok {
			t.Fatalf("expected no frame, got %s", string(data))
		}
	case <-time.After(d):
	}
}

func join(t *testing.T, h *Hub, name, room string) *Client {
	t.Helper()
	c := NewClient(h, name, room, 64)
	if !h.Register(c) {
		t.Fatalf("hub refused registration")
	}
	return c
}

// register two clients -> both receive presence; membership correct.
func TestRegisterPresence(t *testing.T) {
	h := startHub(t)
	a := join(t, h, "alice", "general")
	pa := recvType(t, a, TypePresence)
	if len(pa.Members) != 1 || pa.Members[0] != "alice" {
		t.Fatalf("alice presence = %v, want [alice]", pa.Members)
	}

	b := join(t, h, "bob", "general")
	pb := recvType(t, b, TypePresence)
	if len(pb.Members) != 2 {
		t.Fatalf("bob presence = %v, want 2 members", pb.Members)
	}
	// alice should also receive an updated presence listing both.
	pa2 := recvType(t, a, TypePresence)
	if len(pa2.Members) != 2 {
		t.Fatalf("alice updated presence = %v, want 2 members", pa2.Members)
	}
}

// Broadcast from A reaches B but not A.
func TestBroadcastExcludesSender(t *testing.T) {
	h := startHub(t)
	a := join(t, h, "alice", "general")
	b := join(t, h, "bob", "general")
	settle(a)
	settle(b)

	h.Message(a, "hello")

	got := recvType(t, b, TypeMessage)
	if got.From != "alice" || got.Text != "hello" {
		t.Fatalf("bob got %+v, want from=alice text=hello", got)
	}
	expectSilence(t, a, 200*time.Millisecond) // sender excluded
}

// unregister removes from room and emits system leave + presence.
func TestUnregisterNotifies(t *testing.T) {
	h := startHub(t)
	a := join(t, h, "alice", "general")
	b := join(t, h, "bob", "general")
	settle(a)
	settle(b)

	h.Unregister(b)

	sys := recvType(t, a, TypeSystem)
	if sys.Text != "bob left" {
		t.Fatalf("system text = %q, want 'bob left'", sys.Text)
	}
	pres := recvType(t, a, TypePresence)
	if len(pres.Members) != 1 || pres.Members[0] != "alice" {
		t.Fatalf("presence = %v, want [alice]", pres.Members)
	}
}

// A slow consumer is dropped without blocking the hub or a healthy client.
func TestSlowConsumerDropped(t *testing.T) {
	h := startHub(t)
	// Small buffer so the slow client overflows quickly. It still has room for
	// the few registration frames.
	slow := NewClient(h, "slow", "general", 8)
	if !h.Register(slow) {
		t.Fatal("register slow failed")
	}
	// Healthy receiver with a generous buffer (drained after the burst).
	healthy := NewClient(h, "healthy", "general", 4096)
	if !h.Register(healthy) {
		t.Fatal("register healthy failed")
	}
	// A separate sender so neither receiver is excluded as the originator.
	sender := NewClient(h, "sender", "general", 1024)
	if !h.Register(sender) {
		t.Fatal("register sender failed")
	}

	const n = 200
	for i := 0; i < n; i++ {
		h.Message(sender, "spam")
	}

	// The healthy client (drained continuously) keeps receiving.
	healthyGot := 0
	deadline := time.After(readTimeout)
drainHealthy:
	for healthyGot < n {
		select {
		case data, ok := <-healthy.send:
			if !ok {
				t.Fatal("healthy client was closed")
			}
			var out Outgoing
			if json.Unmarshal(data, &out) == nil && out.Type == TypeMessage {
				healthyGot++
			}
		case <-deadline:
			break drainHealthy
		}
	}
	if healthyGot == 0 {
		t.Fatal("healthy client received nothing")
	}

	// The slow client must eventually be dropped: its send channel is closed.
	if !drainUntilClosed(slow.send, readTimeout) {
		t.Fatal("slow consumer was not dropped")
	}
}

// A duplicate username gets a suffix.
func TestDuplicateUsername(t *testing.T) {
	h := startHub(t)
	a := join(t, h, "bob", "general")
	ackA := recvType(t, a, TypeJoin)
	if ackA.From != "bob" {
		t.Fatalf("first join name = %q, want bob", ackA.From)
	}
	b := join(t, h, "bob", "general")
	ackB := recvType(t, b, TypeJoin)
	if ackB.From != "bob-2" {
		t.Fatalf("duplicate join name = %q, want bob-2", ackB.From)
	}
}

// room isolation: a message in roomA never reaches roomB.
func TestRoomIsolation(t *testing.T) {
	h := startHub(t)
	a := join(t, h, "alice", "roomA")
	b := join(t, h, "bob", "roomB")
	settle(a)
	settle(b)

	h.Message(a, "secret")
	expectSilence(t, b, 300*time.Millisecond)
}

// fan-out under load (run with -race). N senders x M messages each; every
// peer must receive (N-1)*M message frames.
func TestFanOutUnderLoad(t *testing.T) {
	h := startHub(t)
	const (
		nClients = 5
		nMsgs    = 20
	)
	clients := make([]*Client, nClients)
	for i := range clients {
		clients[i] = NewClient(h, names[i], "general", 2048)
		if !h.Register(clients[i]) {
			t.Fatal("register failed")
		}
	}

	want := (nClients - 1) * nMsgs
	var wg sync.WaitGroup

	// Drainers: count message frames per client.
	counts := make([]int, nClients)
	for i, c := range clients {
		wg.Add(1)
		go func(i int, c *Client) {
			defer wg.Done()
			deadline := time.After(5 * time.Second)
			for counts[i] < want {
				select {
				case data, ok := <-c.send:
					if !ok {
						return
					}
					var out Outgoing
					if json.Unmarshal(data, &out) == nil && out.Type == TypeMessage {
						counts[i]++
					}
				case <-deadline:
					return
				}
			}
		}(i, c)
	}

	// Senders.
	var sendWg sync.WaitGroup
	for _, c := range clients {
		sendWg.Add(1)
		go func(c *Client) {
			defer sendWg.Done()
			for j := 0; j < nMsgs; j++ {
				h.Message(c, "x")
			}
		}(c)
	}
	sendWg.Wait()
	wg.Wait()

	for i := range clients {
		if counts[i] != want {
			t.Errorf("client %d received %d messages, want %d", i, counts[i], want)
		}
	}
}

var names = []string{"alice", "bob", "carol", "dave", "erin", "frank"}

// settle drains a client's queue until it has been quiet for a short window,
// providing a deterministic barrier after asynchronous hub registration.
func settle(c *Client) {
	for {
		select {
		case <-c.send:
		case <-time.After(150 * time.Millisecond):
			return
		}
	}
}

func drainUntilClosed(ch chan []byte, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return true
			}
		case <-deadline:
			return false
		}
	}
}
