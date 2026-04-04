// Package crc32 is a table-driven implementation of the IEEE CRC-32 that gzip
// uses for its trailer.
package crc32

const poly = 0xEDB88320 // IEEE polynomial, bit-reversed

var table [256]uint32

func init() {
	for n := 0; n < 256; n++ {
		c := uint32(n)
		for k := 0; k < 8; k++ {
			if c&1 != 0 {
				c = poly ^ (c >> 1)
			} else {
				c >>= 1
			}
		}
		table[n] = c
	}
}

// Update extends a running checksum with the bytes in p.
func Update(crc uint32, p []byte) uint32 {
	crc = ^crc
	for _, b := range p {
		crc = table[byte(crc)^b] ^ (crc >> 8)
	}
	return ^crc
}

// Checksum returns the CRC-32 of p.
func Checksum(p []byte) uint32 {
	return Update(0, p)
}
