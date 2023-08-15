package message

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// Malformed or truncated input always produces one of these rather than a
// panic.
var (
	ErrShortBuffer  = errors.New("dns: buffer too short")
	ErrPointerLoop  = errors.New("dns: compression pointer loop")
	ErrBadPointer   = errors.New("dns: compression pointer out of range")
	ErrReservedBits = errors.New("dns: reserved label bits set")
)

// maxPointerJumps is how many compression pointers readName will follow before
// assuming a loop. Real messages need only a few.
const maxPointerJumps = 128

func Unpack(buf []byte) (*Message, error) {
	if len(buf) < 12 {
		return nil, fmt.Errorf("%w: header needs 12 bytes, have %d", ErrShortBuffer, len(buf))
	}

	m := &Message{}
	m.Header = Header{
		ID:      binary.BigEndian.Uint16(buf[0:2]),
		Flags:   binary.BigEndian.Uint16(buf[2:4]),
		QDCount: binary.BigEndian.Uint16(buf[4:6]),
		ANCount: binary.BigEndian.Uint16(buf[6:8]),
		NSCount: binary.BigEndian.Uint16(buf[8:10]),
		ARCount: binary.BigEndian.Uint16(buf[10:12]),
	}

	off := 12
	var err error

	m.Questions = make([]Question, 0, m.Header.QDCount)
	for i := 0; i < int(m.Header.QDCount); i++ {
		var q Question
		q, off, err = readQuestion(buf, off)
		if err != nil {
			return nil, fmt.Errorf("question %d: %w", i, err)
		}
		m.Questions = append(m.Questions, q)
	}

	if m.Answers, off, err = readRecords(buf, off, int(m.Header.ANCount)); err != nil {
		return nil, fmt.Errorf("answer section: %w", err)
	}
	if m.Authority, off, err = readRecords(buf, off, int(m.Header.NSCount)); err != nil {
		return nil, fmt.Errorf("authority section: %w", err)
	}
	if m.Additional, off, err = readRecords(buf, off, int(m.Header.ARCount)); err != nil {
		return nil, fmt.Errorf("additional section: %w", err)
	}

	return m, nil
}

func readQuestion(buf []byte, off int) (Question, int, error) {
	name, next, err := readName(buf, off)
	if err != nil {
		return Question{}, 0, err
	}
	if next+4 > len(buf) {
		return Question{}, 0, fmt.Errorf("%w: question fixed fields", ErrShortBuffer)
	}
	q := Question{
		Name:  name,
		Type:  binary.BigEndian.Uint16(buf[next : next+2]),
		Class: binary.BigEndian.Uint16(buf[next+2 : next+4]),
	}
	return q, next + 4, nil
}

func readRecords(buf []byte, off, n int) ([]Record, int, error) {
	recs := make([]Record, 0, n)
	for i := 0; i < n; i++ {
		var r Record
		var err error
		r, off, err = readRecord(buf, off)
		if err != nil {
			return nil, 0, fmt.Errorf("record %d: %w", i, err)
		}
		recs = append(recs, r)
	}
	return recs, off, nil
}

func readRecord(buf []byte, off int) (Record, int, error) {
	name, next, err := readName(buf, off)
	if err != nil {
		return Record{}, 0, err
	}
	if next+10 > len(buf) {
		return Record{}, 0, fmt.Errorf("%w: record fixed fields", ErrShortBuffer)
	}
	r := Record{
		Name:     name,
		Type:     binary.BigEndian.Uint16(buf[next : next+2]),
		Class:    binary.BigEndian.Uint16(buf[next+2 : next+4]),
		TTL:      binary.BigEndian.Uint32(buf[next+4 : next+8]),
		RDLength: binary.BigEndian.Uint16(buf[next+8 : next+10]),
		rdataOff: -1,
	}
	rdStart := next + 10
	rdEnd := rdStart + int(r.RDLength)
	if rdEnd > len(buf) {
		return Record{}, 0, fmt.Errorf("%w: rdata (%d bytes) past end", ErrShortBuffer, r.RDLength)
	}
	r.RData = buf[rdStart:rdEnd]
	r.rdataOff = rdStart
	return r, rdEnd, nil
}

// readName decodes the name at off, following compression pointers. It also
// returns the offset just past the name as it appears at off, which is right
// after the first pointer if there was one.
func readName(buf []byte, off int) (string, int, error) {
	var labels []string
	jumps := 0
	next := -1 // set once, at the end of the name or the first pointer
	pos := off

	for {
		if pos < 0 || pos >= len(buf) {
			return "", 0, fmt.Errorf("%w: name at offset %d", ErrShortBuffer, pos)
		}
		b := buf[pos]

		switch b & 0xC0 {
		case 0x00: // length-prefixed label; zero length ends the name
			if b == 0 {
				pos++
				if next < 0 {
					next = pos
				}
				return strings.Join(labels, "."), next, nil
			}
			start := pos + 1
			end := start + int(b)
			if end > len(buf) {
				return "", 0, fmt.Errorf("%w: label at offset %d", ErrShortBuffer, pos)
			}
			labels = append(labels, string(buf[start:end]))
			pos = end
		case 0xC0: // pointer: the low 14 bits of this and the next byte
			if pos+1 >= len(buf) {
				return "", 0, fmt.Errorf("%w: truncated pointer at %d", ErrShortBuffer, pos)
			}
			ptr := int(binary.BigEndian.Uint16(buf[pos:pos+2]) & 0x3FFF)
			if next < 0 {
				next = pos + 2
			}
			jumps++
			if jumps > maxPointerJumps {
				return "", 0, ErrPointerLoop
			}
			if ptr >= len(buf) {
				return "", 0, fmt.Errorf("%w: target %d >= %d", ErrBadPointer, ptr, len(buf))
			}
			pos = ptr
		default: // 0x40 and 0x80 are reserved
			return "", 0, fmt.Errorf("%w: 0x%02x at offset %d", ErrReservedBits, b&0xC0, pos)
		}
	}
}
