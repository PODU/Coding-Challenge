package dhcp

import (
	"encoding/binary"
	"fmt"
)

// Marshal encodes the message. PAD and END in m.Options are ignored; END is
// always written last.
func (m *Message) Marshal() ([]byte, error) {
	b := make([]byte, headerLen)
	b[0] = m.Op
	b[1] = m.HType
	b[2] = m.HLen
	b[3] = m.Hops
	binary.BigEndian.PutUint32(b[4:8], m.XID)
	binary.BigEndian.PutUint16(b[8:10], m.Secs)
	binary.BigEndian.PutUint16(b[10:12], m.Flags)
	copy(b[12:16], ip4(m.CIAddr))
	copy(b[16:20], ip4(m.YIAddr))
	copy(b[20:24], ip4(m.SIAddr))
	copy(b[24:28], ip4(m.GIAddr))

	if len(m.CHAddr) > 16 {
		return nil, fmt.Errorf("dhcp: chaddr length %d exceeds 16", len(m.CHAddr))
	}
	copy(b[28:44], m.CHAddr)
	copy(b[44:108], m.SName[:])
	copy(b[108:236], m.File[:])

	b = append(b, magicCookie...)

	for _, o := range m.Options {
		if o.Code == OptPad || o.Code == OptEnd {
			continue
		}
		if len(o.Data) > 255 {
			return nil, fmt.Errorf("dhcp: option %d data length %d exceeds 255", o.Code, len(o.Data))
		}
		b = append(b, o.Code, byte(len(o.Data)))
		b = append(b, o.Data...)
	}
	b = append(b, OptEnd)
	return b, nil
}
