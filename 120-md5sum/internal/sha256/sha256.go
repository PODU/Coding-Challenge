// Package sha256 implements SHA-256 (FIPS 180-4) for `md5sum -a sha256`.
// Like the md5 package it does not use the standard library hash.
package sha256

import (
	"hash"
	"math/bits"
)

// Size is the length, in bytes, of a SHA-256 checksum.
const Size = 32

// BlockSize is the block size, in bytes, of the SHA-256 hash function.
const BlockSize = 64

// Initial hash values: the fractional parts of the square roots of the first
// eight primes.
var initState = [8]uint32{
	0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
	0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
}

// Round constants: the fractional parts of the cube roots of the first 64 primes.
var k = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5,
	0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3,
	0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc,
	0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
	0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13,
	0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3,
	0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5,
	0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208,
	0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

// Digest is the running state of a SHA-256 computation.
type Digest struct {
	state [8]uint32
	x     [BlockSize]byte
	nx    int
	len   uint64
}

// New returns a new SHA-256 Digest.
func New() *Digest {
	d := &Digest{}
	d.Reset()
	return d
}

// Reset restores the Digest to its initial state.
func (d *Digest) Reset() {
	d.state = initState
	d.nx = 0
	d.len = 0
}

// Size returns 32.
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
		nn := len(p) &^ (BlockSize - 1)
		d.block(p[:nn])
		p = p[nn:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return n, nil
}

// Sum appends the checksum to b without disturbing the running state.
func (d *Digest) Sum(b []byte) []byte {
	dc := *d
	digest := dc.checkSum()
	return append(b, digest[:]...)
}

// checkSum pads like MD5 but with a big-endian length, and returns the final
// state big-endian.
func (d *Digest) checkSum() [Size]byte {
	bitLen := d.len << 3

	var pad [BlockSize + 8]byte
	pad[0] = 0x80
	padLen := 56 - int(d.len%BlockSize)
	if padLen <= 0 {
		padLen += BlockSize
	}

	var lenBytes [8]byte
	putUint64BE(lenBytes[:], bitLen)

	d.Write(pad[:padLen])
	d.Write(lenBytes[:])

	var out [Size]byte
	for i := 0; i < 8; i++ {
		putUint32BE(out[i*4:], d.state[i])
	}
	return out
}

func (d *Digest) block(data []byte) {
	for len(data) >= BlockSize {
		var w [64]uint32
		for i := 0; i < 16; i++ {
			j := i * 4
			w[i] = uint32(data[j])<<24 | uint32(data[j+1])<<16 |
				uint32(data[j+2])<<8 | uint32(data[j+3])
		}
		for i := 16; i < 64; i++ {
			s0 := bits.RotateLeft32(w[i-15], -7) ^ bits.RotateLeft32(w[i-15], -18) ^ (w[i-15] >> 3)
			s1 := bits.RotateLeft32(w[i-2], -17) ^ bits.RotateLeft32(w[i-2], -19) ^ (w[i-2] >> 10)
			w[i] = w[i-16] + s0 + w[i-7] + s1
		}

		aa, bb, cc, dd := d.state[0], d.state[1], d.state[2], d.state[3]
		ee, ff, gg, hh := d.state[4], d.state[5], d.state[6], d.state[7]

		for i := 0; i < 64; i++ {
			S1 := bits.RotateLeft32(ee, -6) ^ bits.RotateLeft32(ee, -11) ^ bits.RotateLeft32(ee, -25)
			ch := (ee & ff) ^ (^ee & gg)
			t1 := hh + S1 + ch + k[i] + w[i]
			S0 := bits.RotateLeft32(aa, -2) ^ bits.RotateLeft32(aa, -13) ^ bits.RotateLeft32(aa, -22)
			maj := (aa & bb) ^ (aa & cc) ^ (bb & cc)
			t2 := S0 + maj

			hh = gg
			gg = ff
			ff = ee
			ee = dd + t1
			dd = cc
			cc = bb
			bb = aa
			aa = t1 + t2
		}

		d.state[0] += aa
		d.state[1] += bb
		d.state[2] += cc
		d.state[3] += dd
		d.state[4] += ee
		d.state[5] += ff
		d.state[6] += gg
		d.state[7] += hh

		data = data[BlockSize:]
	}
}

// Sum returns the SHA-256 checksum of data.
func Sum(data []byte) [Size]byte {
	d := New()
	d.Write(data)
	return d.checkSum()
}

func putUint32BE(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

func putUint64BE(b []byte, v uint64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}

var _ hash.Hash = (*Digest)(nil)
