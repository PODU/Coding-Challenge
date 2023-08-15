// Package message encodes and decodes DNS messages (RFC 1035), including
// compressed names.
package message

import (
	"fmt"
	"net/netip"
	"strings"
)

// Record types. AAAA is from RFC 3596.
const (
	TypeA     uint16 = 1
	TypeNS    uint16 = 2
	TypeCNAME uint16 = 5
	TypeSOA   uint16 = 6
	TypePTR   uint16 = 12
	TypeMX    uint16 = 15
	TypeTXT   uint16 = 16
	TypeAAAA  uint16 = 28
)

const (
	ClassIN uint16 = 1
)

// Response codes.
const (
	RCodeNoError  uint16 = 0
	RCodeFormErr  uint16 = 1
	RCodeServFail uint16 = 2
	RCodeNXDomain uint16 = 3
	RCodeNotImp   uint16 = 4
	RCodeRefused  uint16 = 5
)

// Header is the fixed 12-byte message header.
type Header struct {
	ID      uint16
	Flags   uint16
	QDCount uint16
	ANCount uint16
	NSCount uint16
	ARCount uint16
}

// Bits of Header.Flags.
const (
	flagQR     = 1 << 15 // response
	flagAA     = 1 << 10 // authoritative answer
	flagTC     = 1 << 9  // truncated
	flagRD     = 1 << 8  // recursion desired
	flagRA     = 1 << 7  // recursion available
	maskOpcode = 0x7800  // bits 11-14
	maskRCode  = 0x000F  // bits 0-3
)

// QR reports whether this is a response (QR=1).
func (h Header) QR() bool { return h.Flags&flagQR != 0 }

// AA reports the authoritative-answer bit.
func (h Header) AA() bool { return h.Flags&flagAA != 0 }

// TC reports the truncation bit.
func (h Header) TC() bool { return h.Flags&flagTC != 0 }

// RD reports the recursion-desired bit.
func (h Header) RD() bool { return h.Flags&flagRD != 0 }

// RA reports the recursion-available bit.
func (h Header) RA() bool { return h.Flags&flagRA != 0 }

// Opcode returns the 4-bit opcode.
func (h Header) Opcode() uint16 { return (h.Flags & maskOpcode) >> 11 }

// RCode returns the 4-bit response code.
func (h Header) RCode() uint16 { return h.Flags & maskRCode }

// SetQR sets or clears the QR bit.
func (h *Header) SetQR(v bool) { h.setFlag(flagQR, v) }

// SetRD sets or clears the recursion-desired bit.
func (h *Header) SetRD(v bool) { h.setFlag(flagRD, v) }

func (h *Header) setFlag(mask uint16, v bool) {
	if v {
		h.Flags |= mask
	} else {
		h.Flags &^= mask
	}
}

type Question struct {
	Name  string
	Type  uint16
	Class uint16
}

// Record is a resource record. RData is left undecoded so record types we
// don't understand still survive a round trip.
type Record struct {
	Name     string
	Type     uint16
	Class    uint16
	TTL      uint32
	RDLength uint16
	RData    []byte

	// rdataOff is where RData starts in the message it was unpacked from, or
	// -1. Names inside RData can be compressed, pointing elsewhere in that
	// message, so they have to be decoded against the whole thing.
	rdataOff int
}

type Message struct {
	Header     Header
	Questions  []Question
	Answers    []Record
	Authority  []Record
	Additional []Record
}

// AString interprets the RData of a type-A record as an IPv4 address.
func (r Record) AString() (netip.Addr, bool) {
	if r.Type != TypeA || len(r.RData) != 4 {
		return netip.Addr{}, false
	}
	return netip.AddrFrom4([4]byte{r.RData[0], r.RData[1], r.RData[2], r.RData[3]}), true
}

// AAAAString interprets the RData of a type-AAAA record as an IPv6 address.
func (r Record) AAAAString() (netip.Addr, bool) {
	if r.Type != TypeAAAA || len(r.RData) != 16 {
		return netip.Addr{}, false
	}
	var a [16]byte
	copy(a[:], r.RData)
	return netip.AddrFrom16(a), true
}

// NSString decodes the RData of a type-NS record. msg must be the message the
// record came from, since the name may be compressed.
func (r Record) NSString(msg []byte) (string, bool) {
	if r.Type != TypeNS {
		return "", false
	}
	return r.decodeNameRData(msg)
}

// CNAMEString is NSString for CNAME records.
func (r Record) CNAMEString(msg []byte) (string, bool) {
	if r.Type != TypeCNAME {
		return "", false
	}
	return r.decodeNameRData(msg)
}

func (r Record) decodeNameRData(msg []byte) (string, bool) {
	if r.rdataOff >= 0 && r.rdataOff <= len(msg) {
		name, _, err := readName(msg, r.rdataOff)
		if err == nil {
			return name, true
		}
	}
	// A hand-built record has no message to point into, so its RData must be
	// uncompressed.
	name, _, err := readName(r.RData, 0)
	if err != nil {
		return "", false
	}
	return name, true
}

// TypeName returns the mnemonic for a record type, e.g. "AAAA".
func TypeName(t uint16) string {
	switch t {
	case TypeA:
		return "A"
	case TypeNS:
		return "NS"
	case TypeCNAME:
		return "CNAME"
	case TypeSOA:
		return "SOA"
	case TypePTR:
		return "PTR"
	case TypeMX:
		return "MX"
	case TypeTXT:
		return "TXT"
	case TypeAAAA:
		return "AAAA"
	default:
		return fmt.Sprintf("TYPE%d", t)
	}
}

// TypeFromName parses the record types the CLI supports, ignoring case.
func TypeFromName(s string) (uint16, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "A":
		return TypeA, nil
	case "NS":
		return TypeNS, nil
	case "CNAME":
		return TypeCNAME, nil
	case "AAAA":
		return TypeAAAA, nil
	default:
		return 0, fmt.Errorf("unsupported record type %q", s)
	}
}

// RCodeName returns the mnemonic for a response code, e.g. "NXDOMAIN".
func RCodeName(rc uint16) string {
	switch rc {
	case RCodeNoError:
		return "NOERROR"
	case RCodeFormErr:
		return "FORMERR"
	case RCodeServFail:
		return "SERVFAIL"
	case RCodeNXDomain:
		return "NXDOMAIN"
	case RCodeNotImp:
		return "NOTIMP"
	case RCodeRefused:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE%d", rc)
	}
}
