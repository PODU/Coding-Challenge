package dnsmsg

import "encoding/binary"

type decoder struct {
	buf []byte
	pos int
}

// Decode parses a complete DNS message.
func Decode(buf []byte) (*Message, error) {
	if len(buf) < headerLen {
		return nil, ErrTruncated
	}
	d := &decoder{buf: buf, pos: headerLen}

	var m Message
	m.Header.ID = binary.BigEndian.Uint16(buf[0:2])
	m.Header.setFlags(binary.BigEndian.Uint16(buf[2:4]))
	m.Header.QDCount = binary.BigEndian.Uint16(buf[4:6])
	m.Header.ANCount = binary.BigEndian.Uint16(buf[6:8])
	m.Header.NSCount = binary.BigEndian.Uint16(buf[8:10])
	m.Header.ARCount = binary.BigEndian.Uint16(buf[10:12])

	var err error
	if m.Questions, err = d.questions(int(m.Header.QDCount)); err != nil {
		return nil, err
	}
	if m.Answers, err = d.records(int(m.Header.ANCount)); err != nil {
		return nil, err
	}
	if m.Authorities, err = d.records(int(m.Header.NSCount)); err != nil {
		return nil, err
	}
	if m.Additionals, err = d.records(int(m.Header.ARCount)); err != nil {
		return nil, err
	}
	return &m, nil
}

func (d *decoder) questions(n int) ([]Question, error) {
	if n == 0 {
		return nil, nil
	}
	qs := make([]Question, 0, n)
	for i := 0; i < n; i++ {
		q, err := d.question()
		if err != nil {
			return nil, err
		}
		qs = append(qs, q)
	}
	return qs, nil
}

func (d *decoder) question() (Question, error) {
	name, err := d.name()
	if err != nil {
		return Question{}, err
	}
	if d.pos+4 > len(d.buf) {
		return Question{}, ErrTruncated
	}
	q := Question{
		Name:  name,
		Type:  binary.BigEndian.Uint16(d.buf[d.pos : d.pos+2]),
		Class: binary.BigEndian.Uint16(d.buf[d.pos+2 : d.pos+4]),
	}
	d.pos += 4
	return q, nil
}

func (d *decoder) records(n int) ([]ResourceRecord, error) {
	if n == 0 {
		return nil, nil
	}
	rrs := make([]ResourceRecord, 0, n)
	for i := 0; i < n; i++ {
		rr, err := d.record()
		if err != nil {
			return nil, err
		}
		rrs = append(rrs, rr)
	}
	return rrs, nil
}

func (d *decoder) record() (ResourceRecord, error) {
	name, err := d.name()
	if err != nil {
		return ResourceRecord{}, err
	}
	if d.pos+10 > len(d.buf) {
		return ResourceRecord{}, ErrTruncated
	}
	rr := ResourceRecord{
		Name:  name,
		Type:  binary.BigEndian.Uint16(d.buf[d.pos : d.pos+2]),
		Class: binary.BigEndian.Uint16(d.buf[d.pos+2 : d.pos+4]),
		TTL:   binary.BigEndian.Uint32(d.buf[d.pos+4 : d.pos+8]),
	}
	rdlen := int(binary.BigEndian.Uint16(d.buf[d.pos+8 : d.pos+10]))
	d.pos += 10
	if d.pos+rdlen > len(d.buf) {
		return ResourceRecord{}, ErrTruncated
	}
	rr.RData = append([]byte(nil), d.buf[d.pos:d.pos+rdlen]...)
	d.pos += rdlen
	return rr, nil
}

// name decodes a possibly compressed name at d.pos. d.pos ends up after the
// name's inline bytes: just past the first pointer if there was one, otherwise
// past the terminating zero.
func (d *decoder) name() (string, error) {
	var out []byte
	pos := d.pos
	ptrSeen := false
	jumps := 0
	nameLen := 0

	for {
		if pos >= len(d.buf) {
			return "", ErrTruncated
		}
		b := d.buf[pos]
		switch b & 0xC0 {
		case 0xC0: // compression pointer
			if pos+1 >= len(d.buf) {
				return "", ErrTruncated
			}
			if !ptrSeen {
				d.pos = pos + 2
				ptrSeen = true
			}
			jumps++
			if jumps > maxJumps {
				return "", ErrPointerLoop
			}
			target := int(b&0x3F)<<8 | int(d.buf[pos+1])
			if target >= len(d.buf) {
				return "", ErrBadPointer
			}
			pos = target
		case 0x00: // ordinary label
			length := int(b)
			pos++
			if length == 0 {
				if !ptrSeen {
					d.pos = pos
				}
				return string(out), nil
			}
			if pos+length > len(d.buf) {
				return "", ErrTruncated
			}
			nameLen += length + 1
			if nameLen > maxNameLen {
				return "", ErrNameTooLong
			}
			if len(out) > 0 {
				out = append(out, '.')
			}
			out = append(out, d.buf[pos:pos+length]...)
			pos += length
		default: // 0x40 / 0x80 are reserved label types
			return "", ErrBadLabel
		}
	}
}
