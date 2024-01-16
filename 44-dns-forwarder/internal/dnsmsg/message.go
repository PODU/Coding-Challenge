// Package dnsmsg encodes and decodes RFC 1035 DNS messages.
//
// Every read is bounds checked and compression pointer loops are capped, so a
// malformed datagram returns an error rather than panicking.
package dnsmsg

import "errors"

// Record types (RFC 1035 §3.2.2 and later RFCs).
const (
	TypeA     uint16 = 1
	TypeNS    uint16 = 2
	TypeCNAME uint16 = 5
	TypeSOA   uint16 = 6
	TypePTR   uint16 = 12
	TypeMX    uint16 = 15
	TypeTXT   uint16 = 16
	TypeAAAA  uint16 = 28
	TypeOPT   uint16 = 41 // EDNS0 pseudo-record; its "TTL" field is not a real TTL.
)

// Classes (RFC 1035 §3.2.4).
const (
	ClassIN uint16 = 1
)

// Response codes (RFC 1035 §4.1.1).
const (
	RCodeNoError  uint8 = 0
	RCodeFormErr  uint8 = 1
	RCodeServFail uint8 = 2
	RCodeNXDomain uint8 = 3
	RCodeRefused  uint8 = 5
)

// Opcodes (RFC 1035 §4.1.1).
const (
	OpcodeQuery uint8 = 0
)

const (
	headerLen   = 12
	maxNameLen  = 255
	maxLabelLen = 63
	maxJumps    = 100 // compression-pointer follow limit
)

// Decode and encode errors.
var (
	ErrTruncated   = errors.New("dnsmsg: buffer truncated")
	ErrBadLabel    = errors.New("dnsmsg: invalid label length octet")
	ErrBadPointer  = errors.New("dnsmsg: compression pointer out of range")
	ErrPointerLoop = errors.New("dnsmsg: compression pointer loop")
	ErrNameTooLong = errors.New("dnsmsg: domain name exceeds 255 octets")
	ErrLabelLong   = errors.New("dnsmsg: label exceeds 63 octets")
)

// Header is the 12-byte DNS message header (RFC 1035 §4.1.1).
type Header struct {
	ID      uint16
	QR      bool  // false = query, true = response
	Opcode  uint8 // 4 bits
	AA      bool  // authoritative answer
	TC      bool  // truncated
	RD      bool  // recursion desired
	RA      bool  // recursion available
	Z       uint8 // 3 reserved bits
	RCode   uint8 // 4 bits
	QDCount uint16
	ANCount uint16
	NSCount uint16
	ARCount uint16
}

// flags packs the second 16-bit header word.
func (h Header) flags() uint16 {
	var f uint16
	if h.QR {
		f |= 1 << 15
	}
	f |= uint16(h.Opcode&0x0F) << 11
	if h.AA {
		f |= 1 << 10
	}
	if h.TC {
		f |= 1 << 9
	}
	if h.RD {
		f |= 1 << 8
	}
	if h.RA {
		f |= 1 << 7
	}
	f |= uint16(h.Z&0x07) << 4
	f |= uint16(h.RCode & 0x0F)
	return f
}

// setFlags unpacks the second 16-bit header word into h.
func (h *Header) setFlags(f uint16) {
	h.QR = f&(1<<15) != 0
	h.Opcode = uint8((f >> 11) & 0x0F)
	h.AA = f&(1<<10) != 0
	h.TC = f&(1<<9) != 0
	h.RD = f&(1<<8) != 0
	h.RA = f&(1<<7) != 0
	h.Z = uint8((f >> 4) & 0x07)
	h.RCode = uint8(f & 0x0F)
}

// Question is a single entry of the question section (RFC 1035 §4.1.2). Name is
// the decoded dotted form without a trailing dot ("" for the root).
type Question struct {
	Name  string
	Type  uint16
	Class uint16
}

// ResourceRecord is an answer/authority/additional record (RFC 1035 §4.1.3).
// RData holds the raw, undecoded resource data exactly as it appeared on the
// wire (which may include compression pointers for types such as CNAME/NS).
type ResourceRecord struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	RData []byte
}

// Message is a decoded DNS message.
type Message struct {
	Header      Header
	Questions   []Question
	Answers     []ResourceRecord
	Authorities []ResourceRecord
	Additionals []ResourceRecord
}

// MinAnswerTTL returns the smallest TTL across the answer section, skipping OPT
// pseudo-records. ok is false when there are no cacheable answer records.
func (m *Message) MinAnswerTTL() (ttl uint32, ok bool) {
	for _, rr := range m.Answers {
		if rr.Type == TypeOPT {
			continue
		}
		if !ok || rr.TTL < ttl {
			ttl = rr.TTL
			ok = true
		}
	}
	return ttl, ok
}
