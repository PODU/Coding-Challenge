package ntp

import "testing"

// TestMarshalClientRequest verifies the outgoing request: 48 bytes, first byte
// 0x23 (LI=0, VN=4, Mode=3), transmit timestamp echoed into the buffer.
func TestMarshalClientRequest(t *testing.T) {
	transmit := Timestamp(0xe8676d0040000000)
	req := NewClientRequest(transmit)
	buf := req.Marshal()

	if len(buf) != PacketSize {
		t.Fatalf("len = %d, want %d", len(buf), PacketSize)
	}
	if buf[0] != 0x23 {
		t.Fatalf("first byte = %#02x, want 0x23", buf[0])
	}
	// Transmit timestamp occupies bytes 40..47.
	round, err := Unmarshal(buf)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if round.TransmitTime != transmit {
		t.Fatalf("transmit round-trip = %#016x, want %#016x", uint64(round.TransmitTime), uint64(transmit))
	}
	if round.LeapIndicator() != 0 || round.Version() != 4 || round.Mode() != 3 {
		t.Fatalf("LI/VN/Mode = %d/%d/%d, want 0/4/3", round.LeapIndicator(), round.Version(), round.Mode())
	}
}

// fixtureResponse is a hand-built 48-byte server reply (a captured-style
// fixture). Field offsets are annotated.
var fixtureResponse = []byte{
	0x24, 0x02, 0x0a, 0xe9, // LI=0 VN=4 Mode=4; stratum=2; poll=10; precision=-23
	0x00, 0x00, 0x0c, 0x34, // root delay
	0x00, 0x00, 0x1a, 0x2b, // root dispersion
	0xc0, 0xa8, 0x01, 0x01, // reference id 192.168.1.1
	0xe8, 0x67, 0x6c, 0x00, 0x00, 0x00, 0x00, 0x00, // reference timestamp
	0xe8, 0x67, 0x6d, 0x00, 0x40, 0x00, 0x00, 0x00, // originate (t0)
	0xe8, 0x67, 0x6e, 0x00, 0x80, 0x00, 0x00, 0x00, // receive (t1)
	0xe8, 0x67, 0x6f, 0x00, 0xc0, 0x00, 0x00, 0x00, // transmit (t2)
}

// TestUnmarshalFixture decodes the captured fixture and checks every field.
func TestUnmarshalFixture(t *testing.T) {
	p, err := Unmarshal(fixtureResponse)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	checks := []struct {
		name      string
		got, want uint64
	}{
		{"leap", uint64(p.LeapIndicator()), 0},
		{"version", uint64(p.Version()), 4},
		{"mode", uint64(p.Mode()), 4},
		{"stratum", uint64(p.Stratum), 2},
		{"poll", uint64(uint8(p.Poll)), 10},
		{"precision", uint64(uint8(p.Precision)), 0xe9}, // int8(-23)
		{"rootDelay", uint64(p.RootDelay), 0x00000c34},
		{"rootDispersion", uint64(p.RootDispersion), 0x00001a2b},
		{"refID", uint64(p.ReferenceID), 0xc0a80101},
		{"referenceTime", uint64(p.ReferenceTime), 0xe8676c0000000000},
		{"originateTime", uint64(p.OriginateTime), 0xe8676d0040000000},
		{"receiveTime", uint64(p.ReceiveTime), 0xe8676e0080000000},
		{"transmitTime", uint64(p.TransmitTime), 0xe8676f00c0000000},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %#x, want %#x", c.name, c.got, c.want)
		}
	}
}

// TestUnmarshalBadLength rejects packets that are not exactly 48 bytes.
func TestUnmarshalBadLength(t *testing.T) {
	for _, n := range []int{0, 47, 49, 96} {
		if _, err := Unmarshal(make([]byte, n)); err == nil {
			t.Errorf("Unmarshal(%d bytes) = nil error, want error", n)
		}
	}
}

// TestMarshalUnmarshalRoundTrip confirms a full packet survives a round trip.
func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	orig, err := Unmarshal(fixtureResponse)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	round, err := Unmarshal(orig.Marshal())
	if err != nil {
		t.Fatalf("Unmarshal(Marshal): %v", err)
	}
	if *orig != *round {
		t.Fatalf("round trip mismatch:\n orig=%+v\nround=%+v", orig, round)
	}
}
