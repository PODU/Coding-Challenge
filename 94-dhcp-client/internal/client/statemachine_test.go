package client

import (
	"errors"
	"net"
	"testing"
	"time"

	"dhcpclient/internal/dhcp"
)

func testMAC(t *testing.T) net.HardwareAddr {
	t.Helper()
	m, err := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func newFake(t *testing.T) *fakeServer {
	return &fakeServer{
		t:         t,
		serverID:  net.IPv4(192, 168, 1, 1),
		offerIP:   net.IPv4(192, 168, 1, 50),
		leaseSecs: 3600,
		subnet:    net.IPv4(255, 255, 255, 0),
		router:    net.IPv4(192, 168, 1, 1),
		dns:       net.IPv4(8, 8, 8, 8),
	}
}

func newClient(t *testing.T, f *fakeServer) *Client {
	fixed := time.Unix(1_700_000_000, 0)
	return &Client{
		Transport: f,
		MAC:       testMAC(t),
		XID:       0x3903f326,
		Timeout:   4 * time.Second,
		Retries:   3,
		Now:       func() time.Time { return fixed },
		// Rand nil -> deterministic backoff, no jitter.
	}
}

func TestDORAHappyPath(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f)

	lease, err := c.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Lease assembled from OFFER/ACK.
	if !lease.IP.Equal(net.IPv4(192, 168, 1, 50)) {
		t.Errorf("lease IP = %s", lease.IP)
	}
	if !lease.ServerID.Equal(net.IPv4(192, 168, 1, 1)) {
		t.Errorf("server id = %s", lease.ServerID)
	}
	if lease.LeaseSeconds != 3600 {
		t.Errorf("lease seconds = %d", lease.LeaseSeconds)
	}
	if !lease.SubnetMask.Equal(net.IPv4(255, 255, 255, 0)) {
		t.Errorf("subnet = %s", lease.SubnetMask)
	}
	if len(lease.Routers) != 1 || !lease.Routers[0].Equal(net.IPv4(192, 168, 1, 1)) {
		t.Errorf("routers = %v", lease.Routers)
	}
	if len(lease.DNS) != 1 || !lease.DNS[0].Equal(net.IPv4(8, 8, 8, 8)) {
		t.Errorf("dns = %v", lease.DNS)
	}

	// Client emitted exactly DISCOVER then REQUEST.
	if len(f.sent) != 2 {
		t.Fatalf("sent %d packets, want 2 (DISCOVER, REQUEST)", len(f.sent))
	}
	disc, _ := dhcp.Unmarshal(f.sent[0])
	if mt, _ := disc.MessageType(); mt != dhcp.DISCOVER {
		t.Errorf("first packet message type = %d, want DISCOVER", mt)
	}
	if disc.Flags&dhcp.FlagBroadcast == 0 {
		t.Errorf("DISCOVER broadcast flag not set")
	}
	req, _ := dhcp.Unmarshal(f.sent[1])
	if mt, _ := req.MessageType(); mt != dhcp.REQUEST {
		t.Errorf("second packet message type = %d, want REQUEST", mt)
	}
	if !req.RequestedIP().Equal(net.IPv4(192, 168, 1, 50)) {
		t.Errorf("REQUEST Option 50 = %s, want offered IP", req.RequestedIP())
	}
	if !req.ServerID().Equal(net.IPv4(192, 168, 1, 1)) {
		t.Errorf("REQUEST Option 54 = %s, want server id", req.ServerID())
	}
	if req.XID != disc.XID {
		t.Errorf("xid not constant across DORA: 0x%08x vs 0x%08x", req.XID, disc.XID)
	}
}

func TestNakPathRestarts(t *testing.T) {
	f := newFake(t)
	f.nak = true
	c := newClient(t, f)
	c.MaxRestarts = 1

	_, err := c.Run()
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("err = %v, want ErrRejected", err)
	}
	// Two DORA cycles (initial + 1 restart): DISCOVER, REQUEST, DISCOVER, REQUEST.
	if len(f.sent) != 4 {
		t.Fatalf("sent %d packets, want 4 (two NAK'd cycles)", len(f.sent))
	}
}

func TestTimeoutRetransmitAndGiveUp(t *testing.T) {
	f := newFake(t)
	f.silent = true // server never replies
	c := newClient(t, f)

	_, err := c.Run()
	if !errors.Is(err, ErrNoResponse) {
		t.Fatalf("err = %v, want ErrNoResponse", err)
	}
	// Retries=3 -> attempts 0..3 -> 4 DISCOVER transmissions.
	if len(f.sent) != 4 {
		t.Fatalf("sent %d DISCOVERs, want 4", len(f.sent))
	}
	// Deadlines follow exponential backoff 4s, 8s, 16s, 32s from the fixed clock.
	base := time.Unix(1_700_000_000, 0)
	want := []time.Duration{4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second}
	if len(f.deadlines) != len(want) {
		t.Fatalf("observed %d deadlines, want %d", len(f.deadlines), len(want))
	}
	for i, w := range want {
		if got := f.deadlines[i].Sub(base); got != w {
			t.Errorf("deadline[%d] backoff = %s, want %s", i, got, w)
		}
	}
}

func TestXIDFiltering(t *testing.T) {
	f := newFake(t)
	f.wrongXIDFirst = true // a wrong-xid OFFER precedes the valid one
	c := newClient(t, f)

	lease, err := c.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !lease.IP.Equal(net.IPv4(192, 168, 1, 50)) {
		t.Errorf("lease IP = %s; wrong-xid offer should have been ignored", lease.IP)
	}
}

func TestReleaseFromBound(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f)

	lease := &Lease{
		IP:       net.IPv4(192, 168, 1, 50),
		ServerID: net.IPv4(192, 168, 1, 1),
		XID:      0x3903f326,
		MAC:      testMAC(t),
	}
	if err := c.Release(lease); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if len(f.sent) != 1 {
		t.Fatalf("sent %d packets, want 1 RELEASE", len(f.sent))
	}
	rel, err := dhcp.Unmarshal(f.sent[0])
	if err != nil {
		t.Fatalf("decode RELEASE: %v", err)
	}
	if mt, _ := rel.MessageType(); mt != dhcp.RELEASE {
		t.Errorf("message type = %d, want RELEASE", mt)
	}
	if rel.Flags&dhcp.FlagBroadcast != 0 {
		t.Errorf("RELEASE must be unicast (no broadcast flag), flags=0x%04x", rel.Flags)
	}
	if !rel.CIAddr.Equal(net.IPv4(192, 168, 1, 50)) {
		t.Errorf("RELEASE ciaddr = %s, want the bound IP", rel.CIAddr)
	}
	if !rel.ServerID().Equal(net.IPv4(192, 168, 1, 1)) {
		t.Errorf("RELEASE server id = %s", rel.ServerID())
	}
}

func TestTransitionTable(t *testing.T) {
	cases := []struct {
		state     State
		event     Event
		wantState State
		wantAct   Action
	}{
		{StateInit, EventStart, StateSelecting, ActionSendDiscover},
		{StateSelecting, EventOffer, StateRequesting, ActionSendRequest},
		{StateSelecting, EventTimeout, StateSelecting, ActionSendDiscover},
		{StateRequesting, EventAck, StateBound, ActionBound},
		{StateRequesting, EventNak, StateInit, ActionRestart},
		{StateRequesting, EventTimeout, StateRequesting, ActionSendRequest},
		{StateBound, EventRelease, StateReleased, ActionSendRelease},
		// Unhandled pairs are inert.
		{StateInit, EventOffer, StateInit, ActionNone},
		{StateBound, EventAck, StateBound, ActionNone},
		{StateReleased, EventStart, StateReleased, ActionNone},
	}
	for _, tc := range cases {
		gotState, gotAct := Transition(tc.state, tc.event)
		if gotState != tc.wantState || gotAct != tc.wantAct {
			t.Errorf("Transition(%s, %s) = (%s, %s), want (%s, %s)",
				tc.state, tc.event, gotState, gotAct, tc.wantState, tc.wantAct)
		}
	}
}
