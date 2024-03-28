package ustar

// computeChecksum returns the unsigned and signed byte sums of a header block,
// counting the chksum field itself as spaces. Some old tars wrote the signed
// sum, so readers accept either.
func computeChecksum(block []byte) (unsigned int64, signed int64) {
	for i, b := range block {
		c := b
		if i >= offChksum && i < offChksum+sizeChksum {
			c = ' '
		}
		unsigned += int64(c)
		signed += int64(int8(c))
	}
	return unsigned, signed
}

// writeChecksum fills in the chksum field the traditional way: six octal
// digits, a NUL, then a space.
func writeChecksum(block []byte) {
	unsigned, _ := computeChecksum(block)
	enc, _ := encodeOctal(unsigned, 7)
	field := block[offChksum : offChksum+sizeChksum]
	copy(field, enc)
	field[7] = ' '
}

// verifyChecksum reports whether the stored chksum matches the recomputed
// unsigned or signed sum.
func verifyChecksum(block []byte) bool {
	stored, err := parseOctal(block[offChksum : offChksum+sizeChksum])
	if err != nil {
		return false
	}
	unsigned, signed := computeChecksum(block)
	return stored == unsigned || stored == signed
}
