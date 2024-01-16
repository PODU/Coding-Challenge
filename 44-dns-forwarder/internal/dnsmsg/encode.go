package dnsmsg

import (
	"encoding/binary"
	"strings"
)

// Encode serializes a Message to wire format. Domain names are written
// uncompressed (no 0xC0 pointers), which keeps the encoder simple and the
// output self-contained — suitable for the queries and synthesized responses
// this forwarder generates.
func Encode(m *Message) ([]byte, error) {
	h := m.Header
	h.QDCount = uint16(len(m.Questions))
	h.ANCount = uint16(len(m.Answers))
	h.NSCount = uint16(len(m.Authorities))
	h.ARCount = uint16(len(m.Additionals))

	out := make([]byte, headerLen, headerLen+64)
	binary.BigEndian.PutUint16(out[0:2], h.ID)
	binary.BigEndian.PutUint16(out[2:4], h.flags())
	binary.BigEndian.PutUint16(out[4:6], h.QDCount)
	binary.BigEndian.PutUint16(out[6:8], h.ANCount)
	binary.BigEndian.PutUint16(out[8:10], h.NSCount)
	binary.BigEndian.PutUint16(out[10:12], h.ARCount)

	var err error
	for _, q := range m.Questions {
		if out, err = encodeName(out, q.Name); err != nil {
			return nil, err
		}
		out = appendUint16(out, q.Type)
		out = appendUint16(out, q.Class)
	}
	for _, group := range [][]ResourceRecord{m.Answers, m.Authorities, m.Additionals} {
		for _, rr := range group {
			if out, err = encodeName(out, rr.Name); err != nil {
				return nil, err
			}
			out = appendUint16(out, rr.Type)
			out = appendUint16(out, rr.Class)
			out = appendUint32(out, rr.TTL)
			out = appendUint16(out, uint16(len(rr.RData)))
			out = append(out, rr.RData...)
		}
	}
	return out, nil
}

func encodeName(out []byte, name string) ([]byte, error) {
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return append(out, 0), nil
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" {
			continue // tolerate doubled dots
		}
		if len(label) > maxLabelLen {
			return nil, ErrLabelLong
		}
		out = append(out, byte(len(label)))
		out = append(out, label...)
	}
	return append(out, 0), nil
}

func appendUint16(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}

func appendUint32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// BuildResponse synthesizes a minimal response (no answer records) for the given
// query bytes and response code. It echoes the query ID and question section
// when they can be parsed, and is used to emit FORMERR/SERVFAIL/REFUSED replies
// even for partly malformed queries. It never returns an error.
func BuildResponse(query []byte, rcode uint8) []byte {
	var h Header
	var qsection []byte
	var qdcount uint16

	if len(query) >= headerLen {
		h.ID = binary.BigEndian.Uint16(query[0:2])
		flags := binary.BigEndian.Uint16(query[2:4])
		var qh Header
		qh.setFlags(flags)
		h.Opcode = qh.Opcode
		h.RD = qh.RD

		origQD := binary.BigEndian.Uint16(query[4:6])
		if origQD > 0 {
			d := &decoder{buf: query, pos: headerLen}
			ok := true
			for i := 0; i < int(origQD); i++ {
				if _, err := d.question(); err != nil {
					ok = false
					break
				}
			}
			if ok {
				qsection = append([]byte(nil), query[headerLen:d.pos]...)
				qdcount = origQD
			}
		}
	}

	h.QR = true
	h.RA = true
	h.RCode = rcode
	h.QDCount = qdcount

	out := make([]byte, headerLen, headerLen+len(qsection))
	binary.BigEndian.PutUint16(out[0:2], h.ID)
	binary.BigEndian.PutUint16(out[2:4], h.flags())
	binary.BigEndian.PutUint16(out[4:6], qdcount)
	// AN/NS/AR counts remain zero.
	return append(out, qsection...)
}

// RewriteTTL overwrites every resource-record TTL in the raw message buf with
// remaining, in place. OPT pseudo-records are skipped (their TTL field carries
// EDNS data, not a real TTL). It is used when serving a cached answer so the
// client sees a counting-down TTL. buf is walked defensively.
func RewriteTTL(buf []byte, remaining uint32) error {
	if len(buf) < headerLen {
		return ErrTruncated
	}
	qd := int(binary.BigEndian.Uint16(buf[4:6]))
	an := int(binary.BigEndian.Uint16(buf[6:8]))
	ns := int(binary.BigEndian.Uint16(buf[8:10]))
	ar := int(binary.BigEndian.Uint16(buf[10:12]))

	d := &decoder{buf: buf, pos: headerLen}
	for i := 0; i < qd; i++ {
		if _, err := d.question(); err != nil {
			return err
		}
	}
	for i := 0; i < an+ns+ar; i++ {
		if _, err := d.name(); err != nil {
			return err
		}
		if d.pos+10 > len(buf) {
			return ErrTruncated
		}
		typ := binary.BigEndian.Uint16(buf[d.pos : d.pos+2])
		rdlen := int(binary.BigEndian.Uint16(buf[d.pos+8 : d.pos+10]))
		if typ != TypeOPT {
			binary.BigEndian.PutUint32(buf[d.pos+4:d.pos+8], remaining)
		}
		d.pos += 10 + rdlen
		if d.pos > len(buf) {
			return ErrTruncated
		}
	}
	return nil
}

// SetID rewrites the 16-bit ID field of a raw message in place.
func SetID(buf []byte, id uint16) {
	if len(buf) >= 2 {
		binary.BigEndian.PutUint16(buf[0:2], id)
	}
}

// Truncated reports whether the raw message has the TC (truncation) bit set.
func Truncated(buf []byte) bool {
	if len(buf) < 4 {
		return false
	}
	return binary.BigEndian.Uint16(buf[2:4])&(1<<9) != 0
}
