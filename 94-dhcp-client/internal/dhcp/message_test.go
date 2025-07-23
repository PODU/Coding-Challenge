package dhcp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

func mac(t *testing.T, s string) net.HardwareAddr {
	t.Helper()
	m, err := net.ParseMAC(s)
	if err != nil {
		t.Fatalf("ParseMAC(%q): %v", s, err)
	}
	return m
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		msg  *Message
	}{
		{"discover", BuildDiscover(mac(t, "aa:bb:cc:dd:ee:ff"), 0x3903f326, nil, "host1")},
		{"discover+reqip", BuildDiscover(mac(t, "aa:bb:cc:dd:ee:ff"), 0x11223344, net.IPv4(192, 168, 1, 50), "")},
		{"request", BuildRequest(mac(t, "12:34:56:78:9a:bc"), 0xdeadbeef, net.IPv4(10, 0, 0, 5), net.IPv4(10, 0, 0, 1), "")},
		{"release", BuildRelease(mac(t, "aa:bb:cc:dd:ee:ff"), 0xcafebabe, net.IPv4(10, 0, 0, 5), net.IPv4(10, 0, 0, 1))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := tc.msg.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			got, err := Unmarshal(raw)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if got.Op != tc.msg.Op || got.HType != tc.msg.HType || got.HLen != tc.msg.HLen {
				t.Errorf("header op/htype/hlen mismatch: %+v vs %+v", got, tc.msg)
			}
			if got.XID != tc.msg.XID {
				t.Errorf("xid = 0x%08x, want 0x%08x", got.XID, tc.msg.XID)
			}
			if got.Flags != tc.msg.Flags {
				t.Errorf("flags = 0x%04x, want 0x%04x", got.Flags, tc.msg.Flags)
			}
			if !bytes.Equal(got.CHAddr, tc.msg.CHAddr) {
				t.Errorf("chaddr = %s, want %s", got.CHAddr, tc.msg.CHAddr)
			}
			if !got.CIAddr.Equal(orZero(tc.msg.CIAddr)) {
				t.Errorf("ciaddr = %s, want %s", got.CIAddr, tc.msg.CIAddr)
			}
			if len(got.Options) != len(tc.msg.Options) {
				t.Fatalf("options len = %d, want %d", len(got.Options), len(tc.msg.Options))
			}
			for i := range got.Options {
				if got.Options[i].Code != tc.msg.Options[i].Code ||
					!bytes.Equal(got.Options[i].Data, tc.msg.Options[i].Data) {
					t.Errorf("option %d = %+v, want %+v", i, got.Options[i], tc.msg.Options[i])
				}
			}
		})
	}
}

func orZero(ip net.IP) net.IP {
	if ip == nil {
		return net.IPv4zero.To4()
	}
	return ip
}

// goldenOffer hand-assembles a realistic OFFER on the wire.
func goldenOffer(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteByte(2)                          // op = BOOTREPLY
	b.WriteByte(1)                          // htype = Ethernet
	b.WriteByte(6)                          // hlen
	b.WriteByte(0)                          // hops
	b.Write([]byte{0x39, 0x03, 0xf3, 0x26}) // xid
	b.Write([]byte{0x00, 0x00})             // secs
	b.Write([]byte{0x80, 0x00})             // flags = broadcast
	b.Write([]byte{0, 0, 0, 0})             // ciaddr
	b.Write([]byte{192, 168, 1, 100})       // yiaddr
	b.Write([]byte{192, 168, 1, 1})         // siaddr
	b.Write([]byte{0, 0, 0, 0})             // giaddr
	b.Write([]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	b.Write(make([]byte, 10))  // chaddr padding to 16
	b.Write(make([]byte, 64))  // sname
	b.Write(make([]byte, 128)) // file
	b.Write([]byte{0x63, 0x82, 0x53, 0x63})

	b.Write([]byte{53, 1, 2})                     // message type OFFER
	b.Write([]byte{54, 4, 192, 168, 1, 1})        // server id
	b.Write([]byte{51, 4, 0, 0, 0x0e, 0x10})      // lease 3600
	b.Write([]byte{1, 4, 255, 255, 255, 0})       // subnet mask
	b.Write([]byte{3, 4, 192, 168, 1, 1})         // router
	b.Write([]byte{6, 8, 8, 8, 8, 8, 8, 8, 4, 4}) // DNS 8.8.8.8, 8.8.4.4
	b.WriteByte(255)                              // END
	return b.Bytes()
}

func TestDecodeGoldenOffer(t *testing.T) {
	m, err := Unmarshal(goldenOffer(t))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m.Op != BootReply {
		t.Errorf("op = %d, want %d", m.Op, BootReply)
	}
	if m.XID != 0x3903f326 {
		t.Errorf("xid = 0x%08x", m.XID)
	}
	if !m.YIAddr.Equal(net.IPv4(192, 168, 1, 100)) {
		t.Errorf("yiaddr = %s", m.YIAddr)
	}
	if mt, ok := m.MessageType(); !ok || mt != OFFER {
		t.Errorf("message type = %d ok=%v, want OFFER", mt, ok)
	}
	if !m.ServerID().Equal(net.IPv4(192, 168, 1, 1)) {
		t.Errorf("server id = %s", m.ServerID())
	}
	if lt, ok := m.LeaseTime(); !ok || lt != 3600 {
		t.Errorf("lease = %d ok=%v, want 3600", lt, ok)
	}
	if !m.SubnetMask().Equal(net.IPv4(255, 255, 255, 0)) {
		t.Errorf("subnet = %s", m.SubnetMask())
	}
	routers := m.Routers()
	if len(routers) != 1 || !routers[0].Equal(net.IPv4(192, 168, 1, 1)) {
		t.Errorf("routers = %v", routers)
	}
	dns := m.DNS()
	if len(dns) != 2 || !dns[0].Equal(net.IPv4(8, 8, 8, 8)) || !dns[1].Equal(net.IPv4(8, 8, 4, 4)) {
		t.Errorf("dns = %v", dns)
	}
}

func TestEndiannessAndMagicCookie(t *testing.T) {
	m := BuildDiscover(mac(t, "aa:bb:cc:dd:ee:ff"), 0x01020304, nil, "")
	raw, err := m.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Equal(raw[4:8], []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Errorf("xid bytes = % x, want 01 02 03 04", raw[4:8])
	}
	if !bytes.Equal(raw[10:12], []byte{0x80, 0x00}) {
		t.Errorf("flags bytes = % x, want 80 00", raw[10:12])
	}
	if !bytes.Equal(raw[236:240], []byte{0x63, 0x82, 0x53, 0x63}) {
		t.Errorf("magic cookie = % x", raw[236:240])
	}
}

func TestLeaseTimeBigEndian(t *testing.T) {
	o := OptionLeaseTime(0xFFFFFFFF)
	if !bytes.Equal(o.Data, []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
		t.Errorf("infinite lease = % x", o.Data)
	}
	o = OptionLeaseTime(3600)
	if binary.BigEndian.Uint32(o.Data) != 3600 {
		t.Errorf("lease 3600 = % x", o.Data)
	}
}

func TestUnmarshalMalformed(t *testing.T) {
	valid := goldenOffer(t)

	badCookie := append([]byte(nil), valid...)
	badCookie[236] = 0x00

	// Header + cookie + an option whose length overruns the buffer.
	overrun := append([]byte(nil), valid[:240]...)
	overrun = append(overrun, 53, 10, 2) // claims 10 bytes, only 1 present

	// Code present but no length byte, no END.
	truncLen := append([]byte(nil), valid[:240]...)
	truncLen = append(truncLen, 53)

	// Options present but never terminated by END.
	noEnd := append([]byte(nil), valid[:240]...)
	noEnd = append(noEnd, 53, 1, 2)

	cases := []struct {
		name string
		data []byte
	}{
		{"short", make([]byte, 100)},
		{"empty", nil},
		{"bad-cookie", badCookie},
		{"option-overrun", overrun},
		{"truncated-length", truncLen},
		{"missing-end", noEnd},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Unmarshal(tc.data); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}
