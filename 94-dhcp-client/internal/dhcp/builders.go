package dhcp

import "net"

// DefaultParameterRequestList is the options we ask the server to include.
var DefaultParameterRequestList = []uint8{
	OptSubnetMask, OptRouter, OptDNS, OptLeaseTime, OptServerID,
}

func newBase(mac net.HardwareAddr, xid uint32, flags uint16) *Message {
	return &Message{
		Op:     BootRequest,
		HType:  HTypeEthernet,
		HLen:   HLenEthernet,
		XID:    xid,
		Flags:  flags,
		CHAddr: mac,
	}
}

// BuildDiscover builds a DHCPDISCOVER. requestedIP and hostname may be empty.
func BuildDiscover(mac net.HardwareAddr, xid uint32, requestedIP net.IP, hostname string) *Message {
	m := newBase(mac, xid, FlagBroadcast)
	m.Options = []Option{
		OptionMessageType(DISCOVER),
		OptionClientID(HTypeEthernet, mac),
		OptionParameterRequestList(DefaultParameterRequestList...),
	}
	if requestedIP != nil {
		m.Options = append(m.Options, OptionRequestedIP(requestedIP))
	}
	if hostname != "" {
		m.Options = append(m.Options, OptionHostname(hostname))
	}
	return m
}

// BuildRequest builds a DHCPREQUEST for the offered address from serverID.
func BuildRequest(mac net.HardwareAddr, xid uint32, requestedIP, serverID net.IP, hostname string) *Message {
	m := newBase(mac, xid, FlagBroadcast)
	m.Options = []Option{
		OptionMessageType(REQUEST),
		OptionClientID(HTypeEthernet, mac),
		OptionRequestedIP(requestedIP),
		OptionServerID(serverID),
		OptionParameterRequestList(DefaultParameterRequestList...),
	}
	if hostname != "" {
		m.Options = append(m.Options, OptionHostname(hostname))
	}
	return m
}

// BuildRelease builds a DHCPRELEASE. The client already has an address, so it
// goes in ciaddr and the broadcast flag stays clear.
func BuildRelease(mac net.HardwareAddr, xid uint32, clientIP, serverID net.IP) *Message {
	m := newBase(mac, xid, 0)
	m.CIAddr = clientIP
	m.Options = []Option{
		OptionMessageType(RELEASE),
		OptionClientID(HTypeEthernet, mac),
		OptionServerID(serverID),
	}
	return m
}

// BuildDecline builds a DHCPDECLINE for an offered address that is in use.
func BuildDecline(mac net.HardwareAddr, xid uint32, declinedIP, serverID net.IP) *Message {
	m := newBase(mac, xid, 0)
	m.Options = []Option{
		OptionMessageType(DECLINE),
		OptionClientID(HTypeEthernet, mac),
		OptionRequestedIP(declinedIP),
		OptionServerID(serverID),
	}
	return m
}
