// Package md5 implements MD5 (RFC 1321) without using crypto/md5. Digest
// implements hash.Hash.
package md5

import (
	"hash"
	"math/bits"
)

// Size is the length, in bytes, of an MD5 checksum.
const Size = 16

// BlockSize is the block size, in bytes, of the MD5 hash function.
const BlockSize = 64

// Initial state from RFC 1321.
const (
	initA = 0x67452301
	initB = 0xEFCDAB89
	initC = 0x98BADCFE
	initD = 0x10325476
)

// s holds the per-round left-rotation amounts.
var s = [64]uint{
	7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22,
	5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
	4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23,
	6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
}

// k[i] = floor(2^32 * |sin(i+1)|), as listed in RFC 1321.
var k = [64]uint32{
	0xd76aa478, 0xe8c7b756, 0x242070db, 0xc1bdceee,
	0xf57c0faf, 0x4787c62a, 0xa8304613, 0xfd469501,
	0x698098d8, 0x8b44f7af, 0xffff5bb1, 0x895cd7be,
	0x6b901122, 0xfd987193, 0xa679438e, 0x49b40821,
	0xf61e2562, 0xc040b340, 0x265e5a51, 0xe9b6c7aa,
	0xd62f105d, 0x02441453, 0xd8a1e681, 0xe7d3fbc8,
	0x21e1cde6, 0xc33707d6, 0xf4d50d87, 0x455a14ed,
	0xa9e3e905, 0xfcefa3f8, 0x676f02d9, 0x8d2a4c8a,
	0xfffa3942, 0x8771f681, 0x6d9d6122, 0xfde5380c,
	0xa4beea44, 0x4bdecfa9, 0xf6bb4b60, 0xbebfbc70,
	0x289b7ec6, 0xeaa127fa, 0xd4ef3085, 0x04881d05,
	0xd9d4d039, 0xe6db99e5, 0x1fa27cf8, 0xc4ac5665,
	0xf4292244, 0x432aff97, 0xab9423a7, 0xfc93a039,
	0x655b59c3, 0x8f0ccc92, 0xffeff47d, 0x85845dd1,
	0x6fa87e4f, 0xfe2ce6e0, 0xa3014314, 0x4e0811a1,
	0xf7537e82, 0xbd3af235, 0x2ad7d2bb, 0xeb86d391,
}

// Digest is the running state of an MD5 computation.
type Digest struct {
	state [4]uint32       // A, B, C, D
	x     [BlockSize]byte // partial block
	nx    int             // bytes buffered in x
	len   uint64          // total bytes written
}

// New returns a new MD5 Digest.
func New() *Digest {
	d := &Digest{}
	d.Reset()
	return d
}

// Reset restores the Digest to its initial state.
func (d *Digest) Reset() {
	d.state[0] = initA
	d.state[1] = initB
	d.state[2] = initC
	d.state[3] = initD
	d.nx = 0
	d.len = 0
}

// Size returns 16.
func (d *Digest) Size() int { return Size }

// BlockSize returns 64.
func (d *Digest) BlockSize() int { return BlockSize }

// Write adds p to the digest. It never fails.
func (d *Digest) Write(p []byte) (n int, err error) {
	n = len(p)
	d.len += uint64(n)

	if d.nx > 0 {
		c := copy(d.x[d.nx:], p)
		d.nx += c
		if d.nx == BlockSize {
			d.block(d.x[:])
			d.nx = 0
		}
		p = p[c:]
	}

	if len(p) >= BlockSize {
		nn := len(p) &^ (BlockSize - 1) // whole blocks only
		d.block(p[:nn])
		p = p[nn:]
	}

	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return n, nil
}

// Sum appends the checksum to b. It works on a copy, so the caller can keep
// writing afterwards, as with crypto/md5.
func (d *Digest) Sum(b []byte) []byte {
	dc := *d
	digest := dc.checkSum()
	return append(b, digest[:]...)
}

// checkSum pads the message (0x80, zeros up to 56 mod 64, then the bit length
// as a little-endian uint64) and returns the final state.
func (d *Digest) checkSum() [Size]byte {
	bitLen := d.len << 3

	var pad [BlockSize + 8]byte
	pad[0] = 0x80
	padLen := 56 - int(d.len%BlockSize)
	if padLen <= 0 {
		padLen += BlockSize
	}

	var lenBytes [8]byte
	putUint64LE(lenBytes[:], bitLen)

	d.Write(pad[:padLen])
	d.Write(lenBytes[:])

	var out [Size]byte
	putUint32LE(out[0:], d.state[0])
	putUint32LE(out[4:], d.state[1])
	putUint32LE(out[8:], d.state[2])
	putUint32LE(out[12:], d.state[3])
	return out
}

// block compresses whole 64-byte blocks of data into the state.
func (d *Digest) block(data []byte) {
	a0, b0, c0, d0 := d.state[0], d.state[1], d.state[2], d.state[3]

	for len(data) >= BlockSize {
		var m [16]uint32
		for i := 0; i < 16; i++ {
			j := i * 4
			m[i] = uint32(data[j]) | uint32(data[j+1])<<8 |
				uint32(data[j+2])<<16 | uint32(data[j+3])<<24
		}

		a, b, c, dd := a0, b0, c0, d0

		for i := 0; i < 64; i++ {
			var f uint32
			var g int
			switch {
			case i < 16:
				f = (b & c) | (^b & dd)
				g = i
			case i < 32:
				f = (dd & b) | (^dd & c)
				g = (1 + 5*i) % 16
			case i < 48:
				f = b ^ c ^ dd
				g = (5 + 3*i) % 16
			default:
				f = c ^ (b | ^dd)
				g = (7 * i) % 16
			}
			f = f + a + k[i] + m[g]
			a = dd
			dd = c
			c = b
			b = b + bits.RotateLeft32(f, int(s[i]))
		}

		a0 += a
		b0 += b
		c0 += c
		d0 += dd

		data = data[BlockSize:]
	}

	d.state[0], d.state[1], d.state[2], d.state[3] = a0, b0, c0, d0
}

// Sum returns the MD5 checksum of data as a fixed 16-byte array.
func Sum(data []byte) [Size]byte {
	d := New()
	d.Write(data)
	return d.checkSum()
}

func putUint32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func putUint64LE(b []byte, v uint64) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}

var _ hash.Hash = (*Digest)(nil)
