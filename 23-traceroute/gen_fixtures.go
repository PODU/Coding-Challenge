//go:build ignore

// Generator for the offline ICMP test fixtures. Run with:
//
//	go run gen_fixtures.go
//
// It writes handcrafted, wire-format ICMP byte slices to
// internal/trace/testdata so the parser unit tests are deterministic and need
// no network access.
package main

import (
	"encoding/binary"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const dir = "internal/trace/testdata"

// innerIPv4 builds a minimal 20-byte IPv4 header followed by transport bytes.
func innerIPv4(proto byte, transport []byte) []byte {
	h := make([]byte, 20)
	h[0] = 0x45 // version 4, IHL 5 (20 bytes)
	total := 20 + len(transport)
	binary.BigEndian.PutUint16(h[2:4], uint16(total))
	h[8] = 64    // TTL
	h[9] = proto // protocol
	// src 192.168.0.2 -> dst 93.184.216.34
	copy(h[12:16], []byte{192, 168, 0, 2})
	copy(h[16:20], []byte{93, 184, 216, 34})
	return append(h, transport...)
}

// udpHeader builds the first 8 bytes of a UDP header.
func udpHeader(srcPort, dstPort int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint16(b[0:2], uint16(srcPort))
	binary.BigEndian.PutUint16(b[2:4], uint16(dstPort))
	binary.BigEndian.PutUint16(b[4:6], 8) // length
	return b
}

// icmpEchoHeader builds the first 8 bytes of an ICMP Echo header.
func icmpEchoHeader(typ, id, seq int) []byte {
	b := make([]byte, 8)
	b[0] = byte(typ)
	binary.BigEndian.PutUint16(b[4:6], uint16(id))
	binary.BigEndian.PutUint16(b[6:8], uint16(seq))
	return b
}

func marshal(m *icmp.Message) []byte {
	b, err := m.Marshal(nil)
	if err != nil {
		log.Fatal(err)
	}
	return b
}

func write(name string, b []byte) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d bytes)", path, len(b))
}

func main() {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}

	// 1. Time Exceeded carrying an embedded UDP probe (UDP mode, intermediate
	//    hop). Embedded UDP dst port 33435 (base 33434, ttl 2).
	udpInner := innerIPv4(17, udpHeader(40000, 33435))
	write("time_exceeded_udp.bin", marshal(&icmp.Message{
		Type: ipv4.ICMPTypeTimeExceeded, Code: 0,
		Body: &icmp.TimeExceeded{Data: udpInner},
	}))

	// 2. Destination (port) Unreachable carrying the embedded UDP probe
	//    (UDP mode, destination reached). Code 3 = port unreachable.
	write("port_unreachable.bin", marshal(&icmp.Message{
		Type: ipv4.ICMPTypeDestinationUnreachable, Code: 3,
		Body: &icmp.DstUnreach{Data: innerIPv4(17, udpHeader(40000, 33436))},
	}))

	// 3. Echo Reply with id/seq (ICMP mode, destination reached).
	write("echo_reply.bin", marshal(&icmp.Message{
		Type: ipv4.ICMPTypeEchoReply, Code: 0,
		Body: &icmp.Echo{ID: 0x1234, Seq: 7, Data: []byte("codingchallenges")},
	}))

	// 4. Time Exceeded carrying an embedded ICMP Echo probe (ICMP mode,
	//    intermediate hop). Embedded echo id 0x1234, seq 5.
	icmpInner := innerIPv4(1, icmpEchoHeader(8, 0x1234, 5))
	write("time_exceeded_icmp.bin", marshal(&icmp.Message{
		Type: ipv4.ICMPTypeTimeExceeded, Code: 0,
		Body: &icmp.TimeExceeded{Data: icmpInner},
	}))
}
