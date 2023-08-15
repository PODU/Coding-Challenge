package message

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const (
	maxLabelLen = 63
	maxNameLen  = 255 // encoded, so including the length bytes
)

var ErrNameTooLong = errors.New("dns: encoded name exceeds 255 bytes")

var ErrLabelTooLong = errors.New("dns: label exceeds 63 bytes")

// encodeName encodes a name as length-prefixed labels ending with a zero
// byte, e.g. "dns.google.com" becomes \x03dns\x06google\x03com\x00. It never
// compresses. "" and "." are the root name.
func encodeName(name string) ([]byte, error) {
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return []byte{0}, nil
	}

	out := make([]byte, 0, len(name)+2)
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 {
			return nil, fmt.Errorf("dns: empty label in name %q", name)
		}
		if len(label) > maxLabelLen {
			return nil, fmt.Errorf("%w: %q", ErrLabelTooLong, label)
		}
		out = append(out, byte(len(label)))
		out = append(out, label...)
	}
	out = append(out, 0)

	if len(out) > maxNameLen {
		return nil, fmt.Errorf("%w: %q", ErrNameTooLong, name)
	}
	return out, nil
}

// Pack encodes m. The section counts in the header are taken from the slices,
// not from m.Header.
func (m *Message) Pack() ([]byte, error) {
	buf := make([]byte, 12)
	h := m.Header
	h.QDCount = uint16(len(m.Questions))
	h.ANCount = uint16(len(m.Answers))
	h.NSCount = uint16(len(m.Authority))
	h.ARCount = uint16(len(m.Additional))

	binary.BigEndian.PutUint16(buf[0:2], h.ID)
	binary.BigEndian.PutUint16(buf[2:4], h.Flags)
	binary.BigEndian.PutUint16(buf[4:6], h.QDCount)
	binary.BigEndian.PutUint16(buf[6:8], h.ANCount)
	binary.BigEndian.PutUint16(buf[8:10], h.NSCount)
	binary.BigEndian.PutUint16(buf[10:12], h.ARCount)

	for _, q := range m.Questions {
		name, err := encodeName(q.Name)
		if err != nil {
			return nil, err
		}
		buf = append(buf, name...)
		buf = appendUint16(buf, q.Type)
		buf = appendUint16(buf, q.Class)
	}

	for _, set := range [][]Record{m.Answers, m.Authority, m.Additional} {
		for _, r := range set {
			b, err := packRecord(r)
			if err != nil {
				return nil, err
			}
			buf = append(buf, b...)
		}
	}
	return buf, nil
}

func packRecord(r Record) ([]byte, error) {
	name, err := encodeName(r.Name)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(name)+10+len(r.RData))
	out = append(out, name...)
	out = appendUint16(out, r.Type)
	out = appendUint16(out, r.Class)
	out = appendUint32(out, r.TTL)
	out = appendUint16(out, uint16(len(r.RData)))
	out = append(out, r.RData...)
	return out, nil
}

func appendUint16(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}

func appendUint32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func NewQuery(id uint16, name string, qtype, qclass uint16, recursionDesired bool) *Message {
	m := &Message{
		Questions: []Question{{Name: name, Type: qtype, Class: qclass}},
	}
	m.Header.ID = id
	m.Header.SetRD(recursionDesired)
	return m
}
