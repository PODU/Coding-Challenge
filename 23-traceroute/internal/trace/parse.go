package trace

import (
	"encoding/binary"
	"errors"

	"golang.org/x/net/icmp"
)

// ReplyKind is the type of ICMP message received.
type ReplyKind int

const (
	ReplyUnknown ReplyKind = iota
	ReplyTimeExceeded
	ReplyDestUnreachable
	ReplyEchoReply
)

// ErrShortPacket means the original datagram quoted in an ICMP error was too
// short to read the ports or echo id from.
var ErrShortPacket = errors.New("trace: truncated ICMP payload")

// ParsedReply holds what's needed to match an ICMP message to a probe. For
// errors the ports or echo id come from the quoted original datagram; for an
// echo reply they come from the reply itself.
type ParsedReply struct {
	Kind       ReplyKind
	Code       int
	InnerProto int
	SrcPort    int
	DstPort    int
	ID         int
	Seq        int
}

// ParseICMP decodes an ICMP message starting at the type byte. Malformed input
// returns an error rather than panicking.
func ParseICMP(b []byte) (ParsedReply, error) {
	m, err := icmp.ParseMessage(ProtocolICMP, b)
	if err != nil {
		return ParsedReply{}, err
	}

	r := ParsedReply{Code: m.Code}
	switch body := m.Body.(type) {
	case *icmp.TimeExceeded:
		r.Kind = ReplyTimeExceeded
		if err := parseInner(body.Data, &r); err != nil {
			return ParsedReply{}, err
		}
	case *icmp.DstUnreach:
		r.Kind = ReplyDestUnreachable
		if err := parseInner(body.Data, &r); err != nil {
			return ParsedReply{}, err
		}
	case *icmp.Echo:
		r.Kind = ReplyEchoReply
		r.InnerProto = ProtocolICMP
		r.ID = body.ID
		r.Seq = body.Seq
	default:
		return ParsedReply{}, errors.New("trace: unhandled ICMP message type")
	}
	return r, nil
}

// parseInner reads the quoted original datagram: an IPv4 header followed by at
// least 8 bytes of the transport header. The header is parsed by hand to avoid
// platform byte-order quirks.
func parseInner(data []byte, r *ParsedReply) error {
	if len(data) < 20 {
		return ErrShortPacket
	}
	ihl := int(data[0]&0x0f) * 4
	if ihl < 20 || len(data) < ihl+8 {
		return ErrShortPacket
	}
	r.InnerProto = int(data[9])
	p := data[ihl:]
	switch r.InnerProto {
	case ProtocolUDP:
		r.SrcPort = int(binary.BigEndian.Uint16(p[0:2]))
		r.DstPort = int(binary.BigEndian.Uint16(p[2:4]))
	case ProtocolICMP:
		// Echo header: type, code, checksum, id, seq.
		r.ID = int(binary.BigEndian.Uint16(p[4:6]))
		r.Seq = int(binary.BigEndian.Uint16(p[6:8]))
	default:
		return errors.New("trace: unsupported inner protocol")
	}
	return nil
}

// Matches reports whether r answers the probe identified by id/seq (ICMP mode)
// or dstPort (UDP mode).
func (r ParsedReply) Matches(mode Mode, id, seq, dstPort int) bool {
	switch mode {
	case ModeUDP:
		return r.InnerProto == ProtocolUDP && r.DstPort == dstPort
	case ModeICMP:
		return r.InnerProto == ProtocolICMP && r.ID == id && r.Seq == seq
	default:
		return false
	}
}

// DestinationReached reports whether the probe got to the destination host.
func (r ParsedReply) DestinationReached(mode Mode) bool {
	if mode == ModeICMP {
		return r.Kind == ReplyEchoReply
	}
	// In UDP mode the destination answers with port unreachable (code 3).
	return r.Kind == ReplyDestUnreachable && r.Code == 3
}
