package ntp

import (
	"encoding/binary"
	"fmt"
)

// PacketSize is the size of an NTP header without extensions.
const PacketSize = 48

const (
	Version3 = 3
	Version4 = 4

	ModeClient = 3
	ModeServer = 4

	LeapNoWarning  = 0
	LeapNotInSync  = 3
	MaxStratum     = 15
	KissOfDeathStr = 0 // the stratum of a Kiss-o'-Death packet
)

// Packet is the NTP header. It is big-endian on the wire.
type Packet struct {
	LIVNMode       uint8 // leap indicator (2 bits), version (3), mode (3)
	Stratum        uint8
	Poll           int8
	Precision      int8
	RootDelay      uint32
	RootDispersion uint32
	ReferenceID    uint32
	ReferenceTime  Timestamp
	OriginateTime  Timestamp // t0, copied from our request
	ReceiveTime    Timestamp // t1
	TransmitTime   Timestamp // t2
}

func packLIVNMode(li, vn, mode uint8) uint8 {
	return (li&0x3)<<6 | (vn&0x7)<<3 | (mode & 0x7)
}

// LeapIndicator returns the 2-bit leap indicator.
func (p *Packet) LeapIndicator() uint8 { return (p.LIVNMode >> 6) & 0x3 }

// Version returns the protocol version.
func (p *Packet) Version() uint8 { return (p.LIVNMode >> 3) & 0x7 }

// Mode returns the association mode.
func (p *Packet) Mode() uint8 { return p.LIVNMode & 0x7 }

// NewClientRequest builds a version 4 client request. Its first byte is 0x23.
func NewClientRequest(transmit Timestamp) *Packet {
	return &Packet{
		LIVNMode:     packLIVNMode(LeapNoWarning, Version4, ModeClient),
		TransmitTime: transmit,
	}
}

// Marshal encodes p in wire format.
func (p *Packet) Marshal() []byte {
	b := make([]byte, PacketSize)
	b[0] = p.LIVNMode
	b[1] = p.Stratum
	b[2] = byte(p.Poll)
	b[3] = byte(p.Precision)
	binary.BigEndian.PutUint32(b[4:8], p.RootDelay)
	binary.BigEndian.PutUint32(b[8:12], p.RootDispersion)
	binary.BigEndian.PutUint32(b[12:16], p.ReferenceID)
	binary.BigEndian.PutUint64(b[16:24], uint64(p.ReferenceTime))
	binary.BigEndian.PutUint64(b[24:32], uint64(p.OriginateTime))
	binary.BigEndian.PutUint64(b[32:40], uint64(p.ReceiveTime))
	binary.BigEndian.PutUint64(b[40:48], uint64(p.TransmitTime))
	return b
}

// Unmarshal decodes buf, which must be exactly PacketSize bytes.
func Unmarshal(buf []byte) (*Packet, error) {
	if len(buf) != PacketSize {
		return nil, fmt.Errorf("ntp: invalid packet length %d, want %d", len(buf), PacketSize)
	}
	return &Packet{
		LIVNMode:       buf[0],
		Stratum:        buf[1],
		Poll:           int8(buf[2]),
		Precision:      int8(buf[3]),
		RootDelay:      binary.BigEndian.Uint32(buf[4:8]),
		RootDispersion: binary.BigEndian.Uint32(buf[8:12]),
		ReferenceID:    binary.BigEndian.Uint32(buf[12:16]),
		ReferenceTime:  Timestamp(binary.BigEndian.Uint64(buf[16:24])),
		OriginateTime:  Timestamp(binary.BigEndian.Uint64(buf[24:32])),
		ReceiveTime:    Timestamp(binary.BigEndian.Uint64(buf[32:40])),
		TransmitTime:   Timestamp(binary.BigEndian.Uint64(buf[40:48])),
	}, nil
}

// ReferenceIDString returns the Reference ID as four bytes of text. That is
// only meaningful for stratum 0 and 1; above that it is an address or hash.
func (p *Packet) ReferenceIDString() string {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], p.ReferenceID)
	return string(b[:])
}
