package dhcp

import (
	"encoding/binary"
	"net"
)

// DHCP option codes used by this client (RFC 2132).
const (
	OptPad                  uint8 = 0
	OptSubnetMask           uint8 = 1
	OptRouter               uint8 = 3
	OptDNS                  uint8 = 6
	OptHostname             uint8 = 12
	OptRequestedIP          uint8 = 50
	OptLeaseTime            uint8 = 51
	OptMessageType          uint8 = 53
	OptServerID             uint8 = 54
	OptParameterRequestList uint8 = 55
	OptClientID             uint8 = 61
	OptEnd                  uint8 = 255
)

// Message types carried in option 53.
const (
	DISCOVER uint8 = 1
	OFFER    uint8 = 2
	REQUEST  uint8 = 3
	DECLINE  uint8 = 4
	ACK      uint8 = 5
	NAK      uint8 = 6
	RELEASE  uint8 = 7
	INFORM   uint8 = 8
)

// magicCookie (99.130.83.99) comes right before the options.
var magicCookie = []byte{0x63, 0x82, 0x53, 0x63}

// Option is one code/length/value entry. PAD and END are never stored.
type Option struct {
	Code uint8
	Data []byte
}

// OptionMessageType builds an Option 53 (DHCP Message Type).
func OptionMessageType(t uint8) Option { return Option{Code: OptMessageType, Data: []byte{t}} }

// OptionRequestedIP builds an Option 50 (Requested IP Address).
func OptionRequestedIP(ip net.IP) Option { return Option{Code: OptRequestedIP, Data: ip4(ip)} }

// OptionServerID builds an Option 54 (Server Identifier).
func OptionServerID(ip net.IP) Option { return Option{Code: OptServerID, Data: ip4(ip)} }

// OptionSubnetMask builds an Option 1 (Subnet Mask).
func OptionSubnetMask(ip net.IP) Option { return Option{Code: OptSubnetMask, Data: ip4(ip)} }

// OptionLeaseTime builds an Option 51 (IP Address Lease Time), in seconds.
func OptionLeaseTime(secs uint32) Option {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, secs)
	return Option{Code: OptLeaseTime, Data: b}
}

// OptionHostname builds an Option 12 (Host Name).
func OptionHostname(name string) Option {
	return Option{Code: OptHostname, Data: []byte(name)}
}

// OptionParameterRequestList builds an Option 55 from the given option codes.
func OptionParameterRequestList(codes ...uint8) Option {
	return Option{Code: OptParameterRequestList, Data: append([]byte(nil), codes...)}
}

// OptionClientID builds an Option 61 (Client Identifier): the hardware type
// followed by the address.
func OptionClientID(htype uint8, mac net.HardwareAddr) Option {
	data := make([]byte, 0, 1+len(mac))
	data = append(data, htype)
	data = append(data, mac...)
	return Option{Code: OptClientID, Data: data}
}

// OptionRouters builds an Option 3 from one or more router addresses.
func OptionRouters(ips ...net.IP) Option { return ipListOption(OptRouter, ips) }

// OptionDNS builds an Option 6 from one or more DNS server addresses.
func OptionDNS(ips ...net.IP) Option { return ipListOption(OptDNS, ips) }

func ipListOption(code uint8, ips []net.IP) Option {
	data := make([]byte, 0, 4*len(ips))
	for _, ip := range ips {
		data = append(data, ip4(ip)...)
	}
	return Option{Code: code, Data: data}
}

// ip4 returns ip as 4 bytes, or 0.0.0.0 if it isn't IPv4.
func ip4(ip net.IP) []byte {
	out := make([]byte, 4)
	if v4 := ip.To4(); v4 != nil {
		copy(out, v4)
	}
	return out
}

func parseIPList(data []byte) []net.IP {
	var out []net.IP
	for i := 0; i+4 <= len(data); i += 4 {
		out = append(out, net.IPv4(data[i], data[i+1], data[i+2], data[i+3]).To4())
	}
	return out
}
