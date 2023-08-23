package trace

import (
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// probePayload is the text from the challenge, repeated to fill the packet.
const probePayload = "codingchallenges.fyi trace route"

// BuildPayload returns size bytes of probe payload.
func BuildPayload(size int) []byte {
	if size < 0 {
		size = 0
	}
	b := make([]byte, size)
	for i := range b {
		b[i] = probePayload[i%len(probePayload)]
	}
	return b
}

// DestPort uses the classic traceroute scheme of one port per TTL, so a reply
// tells us which hop it came from.
func DestPort(base, ttl int) int {
	return base + (ttl - 1)
}

// BuildEchoRequest returns an echo request whose total size, including the
// 8-byte header, is size bytes.
func BuildEchoRequest(id, seq, size int) *icmp.Message {
	payloadLen := max(size-8, 0)
	return &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: BuildPayload(payloadLen),
		},
	}
}
