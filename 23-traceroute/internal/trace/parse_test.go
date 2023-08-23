package trace

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseICMP_Classification(t *testing.T) {
	tests := []struct {
		name       string
		fixture    string
		wantKind   ReplyKind
		wantProto  int
		wantDstPrt int
		wantID     int
		wantSeq    int
	}{
		{"udp time exceeded", "time_exceeded_udp.bin", ReplyTimeExceeded, ProtocolUDP, 33435, 0, 0},
		{"udp port unreachable", "port_unreachable.bin", ReplyDestUnreachable, ProtocolUDP, 33436, 0, 0},
		{"echo reply", "echo_reply.bin", ReplyEchoReply, ProtocolICMP, 0, 0x1234, 7},
		{"icmp time exceeded", "time_exceeded_icmp.bin", ReplyTimeExceeded, ProtocolICMP, 0, 0x1234, 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseICMP(loadFixture(t, tc.fixture))
			if err != nil {
				t.Fatalf("ParseICMP error: %v", err)
			}
			if r.Kind != tc.wantKind {
				t.Errorf("Kind = %d, want %d", r.Kind, tc.wantKind)
			}
			if r.InnerProto != tc.wantProto {
				t.Errorf("InnerProto = %d, want %d", r.InnerProto, tc.wantProto)
			}
			if tc.wantDstPrt != 0 && r.DstPort != tc.wantDstPrt {
				t.Errorf("DstPort = %d, want %d", r.DstPort, tc.wantDstPrt)
			}
			if tc.wantID != 0 && r.ID != tc.wantID {
				t.Errorf("ID = %#x, want %#x", r.ID, tc.wantID)
			}
			if tc.wantSeq != 0 && r.Seq != tc.wantSeq {
				t.Errorf("Seq = %d, want %d", r.Seq, tc.wantSeq)
			}
		})
	}
}

func TestParseICMP_DestinationReached(t *testing.T) {
	tests := []struct {
		fixture string
		mode    Mode
		want    bool
	}{
		{"port_unreachable.bin", ModeUDP, true},
		{"time_exceeded_udp.bin", ModeUDP, false},
		{"echo_reply.bin", ModeICMP, true},
		{"time_exceeded_icmp.bin", ModeICMP, false},
	}
	for _, tc := range tests {
		r, err := ParseICMP(loadFixture(t, tc.fixture))
		if err != nil {
			t.Fatalf("%s: %v", tc.fixture, err)
		}
		if got := r.DestinationReached(tc.mode); got != tc.want {
			t.Errorf("%s DestinationReached(%v) = %v, want %v", tc.fixture, tc.mode, got, tc.want)
		}
	}
}

func TestParsedReply_Matches(t *testing.T) {
	udpTE, err := ParseICMP(loadFixture(t, "time_exceeded_udp.bin"))
	if err != nil {
		t.Fatal(err)
	}
	// Embedded UDP dst port is 33435.
	if !udpTE.Matches(ModeUDP, 0, 0, 33435) {
		t.Error("UDP reply should match its destination port 33435")
	}
	if udpTE.Matches(ModeUDP, 0, 0, 33499) {
		t.Error("UDP reply should not match an unrelated destination port")
	}

	echo, err := ParseICMP(loadFixture(t, "echo_reply.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !echo.Matches(ModeICMP, 0x1234, 7, 0) {
		t.Error("echo reply should match its id/seq")
	}
	if echo.Matches(ModeICMP, 0x9999, 7, 0) {
		t.Error("echo reply should not match a mismatched id")
	}
	if echo.Matches(ModeICMP, 0x1234, 99, 0) {
		t.Error("echo reply should not match a mismatched seq")
	}
}

func TestParseICMP_Malformed(t *testing.T) {
	cases := [][]byte{
		nil,
		{},
		{0x0b},                   // type only, truncated
		{0x0b, 0x00, 0x00, 0x00}, // time exceeded header without body
	}
	for i, b := range cases {
		if _, err := ParseICMP(b); err == nil {
			t.Errorf("case %d: expected error for malformed input, got nil", i)
		}
	}
}

func TestParseICMP_ShortInnerHeader(t *testing.T) {
	// A well-formed ICMP TimeExceeded whose embedded datagram is too short to
	// contain an IP header + 8 transport bytes should error, not panic.
	short := []byte{0x0b, 0x00, 0x00, 0x00, 0, 0, 0, 0, 0x45, 0x00} // 2 bytes of "inner"
	if _, err := ParseICMP(short); err == nil {
		t.Error("expected error for short inner header")
	}
}
