package peer

// Bitfield records which pieces a peer has. The high bit of byte 0 is piece 0.
type Bitfield []byte

// HasPiece reports whether piece index is set.
func (bf Bitfield) HasPiece(index int) bool {
	if index < 0 || index/8 >= len(bf) {
		return false
	}
	return bf[index/8]>>(7-index%8)&1 == 1
}

// SetPiece sets piece index. Out-of-range indices are ignored.
func (bf Bitfield) SetPiece(index int) {
	if index < 0 || index/8 >= len(bf) {
		return
	}
	bf[index/8] |= 1 << (7 - index%8)
}
