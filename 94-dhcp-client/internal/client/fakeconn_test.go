package client

import (
	"net"
	"testing"
	"time"

	"dhcpclient/internal/dhcp"
)

// fakeServer is an in-process DHCP server implementing Transport. It decodes
// the client's DISCOVER/REQUEST and enqueues canned OFFER/ACK/NAK replies, so a
// full DORA cycle runs entirely in memory with no real network.
type fakeServer struct {
	t *testing.T

	serverID  net.IP
	offerIP   net.IP
	leaseSecs uint32
	subnet    net.IP
	router    net.IP
	dns       net.IP

	nak           bool // answer REQUEST with NAK instead of ACK
	silent        bool // never enqueue replies (simulate a dead server)
	wrongXIDFirst bool // enqueue a bogus-xid OFFER before the real one
	timeouts      int  // emit this many ErrTimeouts before serving the queue

	outbox    [][]byte    // packets queued for the client
	sent      [][]byte    // raw packets the client sent
	deadlines []time.Time // receive deadlines observed (for backoff assertions)
}

func (f *fakeServer) Send(b []byte) error {
	f.sent = append(f.sent, append([]byte(nil), b...))
	if f.silent {
		return nil
	}
	msg, err := dhcp.Unmarshal(b)
	if err != nil {
		f.t.Fatalf("fake server: client sent undecodable packet: %v", err)
	}
	mt, ok := msg.MessageType()
	if !ok {
		f.t.Fatalf("fake server: client packet has no message type")
	}
	switch mt {
	case dhcp.DISCOVER:
		if f.wrongXIDFirst {
			f.outbox = append(f.outbox, f.buildOffer(msg.XID^0xFFFFFFFF, msg.CHAddr))
		}
		f.outbox = append(f.outbox, f.buildOffer(msg.XID, msg.CHAddr))
	case dhcp.REQUEST:
		if f.nak {
			f.outbox = append(f.outbox, f.buildNak(msg.XID, msg.CHAddr))
		} else {
			f.outbox = append(f.outbox, f.buildAck(msg.XID, msg.CHAddr))
		}
	case dhcp.RELEASE:
		// no reply to a RELEASE
	}
	return nil
}

func (f *fakeServer) Receive(deadline time.Time) ([]byte, error) {
	f.deadlines = append(f.deadlines, deadline)
	if f.timeouts > 0 {
		f.timeouts--
		return nil, ErrTimeout
	}
	if len(f.outbox) == 0 {
		return nil, ErrTimeout
	}
	p := f.outbox[0]
	f.outbox = f.outbox[1:]
	return p, nil
}

func (f *fakeServer) Close() error { return nil }

func (f *fakeServer) reply(xid uint32, mac net.HardwareAddr, mt uint8) *dhcp.Message {
	m := &dhcp.Message{
		Op:     dhcp.BootReply,
		HType:  dhcp.HTypeEthernet,
		HLen:   dhcp.HLenEthernet,
		XID:    xid,
		YIAddr: f.offerIP,
		SIAddr: f.serverID,
		CHAddr: mac,
		Options: []dhcp.Option{
			dhcp.OptionMessageType(mt),
			dhcp.OptionServerID(f.serverID),
		},
	}
	return m
}

func (f *fakeServer) buildOffer(xid uint32, mac net.HardwareAddr) []byte {
	m := f.reply(xid, mac, dhcp.OFFER)
	m.Options = append(m.Options,
		dhcp.OptionLeaseTime(f.leaseSecs),
		dhcp.OptionSubnetMask(f.subnet),
		dhcp.OptionRouters(f.router),
		dhcp.OptionDNS(f.dns),
	)
	return mustMarshal(f.t, m)
}

func (f *fakeServer) buildAck(xid uint32, mac net.HardwareAddr) []byte {
	m := f.reply(xid, mac, dhcp.ACK)
	m.Options = append(m.Options,
		dhcp.OptionLeaseTime(f.leaseSecs),
		dhcp.OptionSubnetMask(f.subnet),
		dhcp.OptionRouters(f.router),
		dhcp.OptionDNS(f.dns),
	)
	return mustMarshal(f.t, m)
}

func (f *fakeServer) buildNak(xid uint32, mac net.HardwareAddr) []byte {
	m := &dhcp.Message{
		Op:     dhcp.BootReply,
		HType:  dhcp.HTypeEthernet,
		HLen:   dhcp.HLenEthernet,
		XID:    xid,
		CHAddr: mac,
		Options: []dhcp.Option{
			dhcp.OptionMessageType(dhcp.NAK),
			dhcp.OptionServerID(f.serverID),
		},
	}
	return mustMarshal(f.t, m)
}

func mustMarshal(t *testing.T, m *dhcp.Message) []byte {
	t.Helper()
	b, err := m.Marshal()
	if err != nil {
		t.Fatalf("marshal reply: %v", err)
	}
	return b
}
