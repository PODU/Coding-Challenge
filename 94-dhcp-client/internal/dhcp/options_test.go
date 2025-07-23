package dhcp

import (
	"bytes"
	"net"
	"testing"
)

func TestOptionConstructors(t *testing.T) {
	if o := OptionMessageType(DISCOVER); o.Code != OptMessageType || !bytes.Equal(o.Data, []byte{1}) {
		t.Errorf("message type option = %+v", o)
	}
	if o := OptionServerID(net.IPv4(10, 0, 0, 1)); !bytes.Equal(o.Data, []byte{10, 0, 0, 1}) {
		t.Errorf("server id option = % x", o.Data)
	}
	if o := OptionParameterRequestList(1, 3, 6); !bytes.Equal(o.Data, []byte{1, 3, 6}) {
		t.Errorf("param request list = % x", o.Data)
	}
	if o := OptionClientID(HTypeEthernet, net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}); !bytes.Equal(o.Data, []byte{1, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}) {
		t.Errorf("client id = % x", o.Data)
	}
}

func TestMultiIPOptionsRoundTrip(t *testing.T) {
	routers := OptionRouters(net.IPv4(192, 168, 1, 1), net.IPv4(192, 168, 1, 2))
	if len(routers.Data) != 8 {
		t.Fatalf("router data len = %d, want 8", len(routers.Data))
	}
	got := parseIPList(routers.Data)
	if len(got) != 2 || !got[0].Equal(net.IPv4(192, 168, 1, 1)) || !got[1].Equal(net.IPv4(192, 168, 1, 2)) {
		t.Errorf("parsed routers = %v", got)
	}

	dns := OptionDNS(net.IPv4(8, 8, 8, 8), net.IPv4(1, 1, 1, 1))
	parsed := parseIPList(dns.Data)
	if len(parsed) != 2 {
		t.Errorf("parsed dns = %v", parsed)
	}
}

func TestPadAndEndHandling(t *testing.T) {
	// Build a minimal message and inject PAD bytes between options.
	var b bytes.Buffer
	header := make([]byte, 236) // hlen 0 -> empty chaddr; valid for option parsing
	b.Write(header)
	b.Write([]byte{0x63, 0x82, 0x53, 0x63})
	b.WriteByte(OptPad) // pad
	b.WriteByte(OptPad)
	b.Write([]byte{53, 1, 2}) // OFFER
	b.WriteByte(OptPad)
	b.Write([]byte{54, 4, 10, 0, 0, 1})
	b.WriteByte(255)
	b.Write([]byte{0, 0, 0}) // trailing padding after END (ignored)

	m, err := Unmarshal(b.Bytes())
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if mt, ok := m.MessageType(); !ok || mt != OFFER {
		t.Errorf("message type = %d ok=%v", mt, ok)
	}
	if !m.ServerID().Equal(net.IPv4(10, 0, 0, 1)) {
		t.Errorf("server id = %s", m.ServerID())
	}
	if len(m.Options) != 2 {
		t.Errorf("options = %d, want 2 (pads skipped)", len(m.Options))
	}
}
