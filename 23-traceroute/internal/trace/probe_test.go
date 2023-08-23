package trace

import (
	"bytes"
	"testing"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func TestBuildPayload_Size(t *testing.T) {
	for _, size := range []int{0, 1, 32, 33, 100} {
		got := BuildPayload(size)
		if len(got) != size {
			t.Errorf("BuildPayload(%d) length = %d, want %d", size, len(got), size)
		}
	}
	if got := BuildPayload(-5); len(got) != 0 {
		t.Errorf("BuildPayload(negative) length = %d, want 0", len(got))
	}
}

func TestBuildPayload_PatternRepeats(t *testing.T) {
	got := BuildPayload(len(probePayload) + 3)
	want := append([]byte(probePayload), probePayload[:3]...)
	if !bytes.Equal(got, want) {
		t.Errorf("payload = %q, want %q", got, want)
	}
}

func TestDestPort(t *testing.T) {
	tests := []struct {
		ttl  int
		want int
	}{
		{1, 33434},
		{2, 33435},
		{64, 33497},
	}
	for _, tc := range tests {
		if got := DestPort(DefaultBasePort, tc.ttl); got != tc.want {
			t.Errorf("DestPort(%d, %d) = %d, want %d", DefaultBasePort, tc.ttl, got, tc.want)
		}
	}
}

func TestBuildEchoRequest(t *testing.T) {
	const id, seq, size = 0xABCD, 42, 32
	msg := BuildEchoRequest(id, seq, size)

	if msg.Type != ipv4.ICMPTypeEcho {
		t.Errorf("Type = %v, want Echo", msg.Type)
	}
	echo, ok := msg.Body.(*icmp.Echo)
	if !ok {
		t.Fatalf("Body type = %T, want *icmp.Echo", msg.Body)
	}
	if echo.ID != id || echo.Seq != seq {
		t.Errorf("ID/Seq = %#x/%d, want %#x/%d", echo.ID, echo.Seq, id, seq)
	}

	// Marshalled message must be exactly `size` bytes (8 header + payload) and
	// round-trip back through the parser to the same id/seq.
	b, err := msg.Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != size {
		t.Errorf("marshalled length = %d, want %d", len(b), size)
	}
	parsed, err := ParseICMP(b)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Kind != ReplyUnknown && parsed.ID != id {
		t.Errorf("round-trip ID = %#x, want %#x", parsed.ID, id)
	}
}

func TestBuildEchoRequest_SmallSize(t *testing.T) {
	// Sizes below the 8-byte header must not produce a negative payload.
	msg := BuildEchoRequest(1, 1, 4)
	echo := msg.Body.(*icmp.Echo)
	if len(echo.Data) != 0 {
		t.Errorf("payload length = %d, want 0 for undersized packet", len(echo.Data))
	}
}
