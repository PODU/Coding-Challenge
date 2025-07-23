// Package dhcp encodes and decodes DHCP messages (RFC 2131, options from
// RFC 2132).
package dhcp

import (
	"encoding/binary"
	"net"
)

const (
	BootRequest uint8 = 1 // client -> server
	BootReply   uint8 = 2 // server -> client
)

const (
	HTypeEthernet uint8 = 1
	HLenEthernet  uint8 = 6
)

// FlagBroadcast asks the server to broadcast its reply, which a client with
// no address yet needs.
const FlagBroadcast uint16 = 0x8000

const (
	headerLen = 236
	minLen    = headerLen + 4 // header plus magic cookie
)

// Message is a DHCP message. Field names follow RFC 2131.
type Message struct {
	Op     uint8
	HType  uint8
	HLen   uint8
	Hops   uint8
	XID    uint32
	Secs   uint16
	Flags  uint16
	CIAddr net.IP // client's current address
	YIAddr net.IP // address offered to the client
	SIAddr net.IP
	GIAddr net.IP // relay agent

	CHAddr net.HardwareAddr
	SName  [64]byte
	File   [128]byte

	Options []Option
}

func (m *Message) option(code uint8) ([]byte, bool) {
	for _, o := range m.Options {
		if o.Code == code {
			return o.Data, true
		}
	}
	return nil, false
}

// MessageType returns option 53, if present.
func (m *Message) MessageType() (uint8, bool) {
	if d, ok := m.option(OptMessageType); ok && len(d) >= 1 {
		return d[0], true
	}
	return 0, false
}

// ServerID returns option 54, or nil.
func (m *Message) ServerID() net.IP {
	if d, ok := m.option(OptServerID); ok && len(d) >= 4 {
		return net.IPv4(d[0], d[1], d[2], d[3]).To4()
	}
	return nil
}

// RequestedIP returns option 50, or nil.
func (m *Message) RequestedIP() net.IP {
	if d, ok := m.option(OptRequestedIP); ok && len(d) >= 4 {
		return net.IPv4(d[0], d[1], d[2], d[3]).To4()
	}
	return nil
}

// LeaseTime returns option 51 in seconds, if present.
func (m *Message) LeaseTime() (uint32, bool) {
	if d, ok := m.option(OptLeaseTime); ok && len(d) >= 4 {
		return binary.BigEndian.Uint32(d), true
	}
	return 0, false
}

// SubnetMask returns option 1, or nil.
func (m *Message) SubnetMask() net.IP {
	if d, ok := m.option(OptSubnetMask); ok && len(d) >= 4 {
		return net.IPv4(d[0], d[1], d[2], d[3]).To4()
	}
	return nil
}

// Routers returns option 3.
func (m *Message) Routers() []net.IP {
	if d, ok := m.option(OptRouter); ok {
		return parseIPList(d)
	}
	return nil
}

// DNS returns option 6.
func (m *Message) DNS() []net.IP {
	if d, ok := m.option(OptDNS); ok {
		return parseIPList(d)
	}
	return nil
}
