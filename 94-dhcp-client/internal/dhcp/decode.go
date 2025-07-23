package dhcp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

// Unmarshal decodes a DHCP packet. Malformed input returns an error, never a
// panic.
func Unmarshal(data []byte) (*Message, error) {
	if len(data) < minLen {
		return nil, fmt.Errorf("dhcp: message too short: %d bytes (need >= %d)", len(data), minLen)
	}

	m := &Message{
		Op:    data[0],
		HType: data[1],
		HLen:  data[2],
		Hops:  data[3],
		XID:   binary.BigEndian.Uint32(data[4:8]),
		Secs:  binary.BigEndian.Uint16(data[8:10]),
		Flags: binary.BigEndian.Uint16(data[10:12]),
	}
	m.CIAddr = net.IPv4(data[12], data[13], data[14], data[15]).To4()
	m.YIAddr = net.IPv4(data[16], data[17], data[18], data[19]).To4()
	m.SIAddr = net.IPv4(data[20], data[21], data[22], data[23]).To4()
	m.GIAddr = net.IPv4(data[24], data[25], data[26], data[27]).To4()

	hlen := int(m.HLen)
	if hlen > 16 {
		return nil, fmt.Errorf("dhcp: hlen %d exceeds 16", hlen)
	}
	m.CHAddr = net.HardwareAddr(append([]byte(nil), data[28:28+hlen]...))
	copy(m.SName[:], data[44:108])
	copy(m.File[:], data[108:236])

	if !bytes.Equal(data[236:240], magicCookie) {
		return nil, errors.New("dhcp: bad magic cookie (not a DHCP message)")
	}

	opts, err := parseOptions(data[240:])
	if err != nil {
		return nil, err
	}
	m.Options = opts
	return m, nil
}

// parseOptions reads options up to END, skipping PAD. A missing END is an
// error.
func parseOptions(data []byte) ([]Option, error) {
	var opts []Option
	i := 0
	sawEnd := false
	for i < len(data) {
		code := data[i]
		i++
		if code == OptEnd {
			sawEnd = true
			break
		}
		if code == OptPad {
			continue
		}
		if i >= len(data) {
			return nil, fmt.Errorf("dhcp: truncated option %d: missing length byte at offset %d", code, i)
		}
		length := int(data[i])
		i++
		if i+length > len(data) {
			return nil, fmt.Errorf("dhcp: option %d length %d overruns buffer at offset %d", code, length, i)
		}
		val := append([]byte(nil), data[i:i+length]...)
		i += length
		opts = append(opts, Option{Code: code, Data: val})
	}
	if !sawEnd {
		return nil, errors.New("dhcp: options not terminated by END (255)")
	}
	return opts, nil
}
