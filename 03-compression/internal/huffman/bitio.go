package huffman

import "io"

// BitWriter packs bits MSB-first into bytes.
type BitWriter struct {
	w     io.Writer
	cur   byte
	nbits uint8 // bits buffered in cur
	buf   [1]byte
}

// NewBitWriter wraps w. Callers should pass a buffered writer.
func NewBitWriter(w io.Writer) *BitWriter { return &BitWriter{w: w} }

// WriteBits writes the low n bits of v, most-significant bit first.
func (bw *BitWriter) WriteBits(v uint64, n uint8) error {
	for n > 0 {
		n--
		bw.cur = bw.cur<<1 | byte(v>>n&1)
		bw.nbits++
		if bw.nbits == 8 {
			bw.buf[0] = bw.cur
			if _, err := bw.w.Write(bw.buf[:]); err != nil {
				return err
			}
			bw.cur = 0
			bw.nbits = 0
		}
	}
	return nil
}

// Close flushes a partial last byte, padded with zero bits. The padding is
// never decoded because the header records how many symbols there are.
func (bw *BitWriter) Close() error {
	if bw.nbits > 0 {
		bw.cur <<= (8 - bw.nbits)
		bw.buf[0] = bw.cur
		if _, err := bw.w.Write(bw.buf[:]); err != nil {
			return err
		}
		bw.cur = 0
		bw.nbits = 0
	}
	return nil
}

// BitReader reads bits MSB-first.
type BitReader struct {
	r     io.Reader
	cur   byte
	nbits uint8 // unread bits left in cur
	buf   [1]byte
}

// NewBitReader wraps r. Callers should pass a buffered reader.
func NewBitReader(r io.Reader) *BitReader { return &BitReader{r: r} }

// ReadBit returns the next bit, or io.EOF once the input runs out.
func (br *BitReader) ReadBit() (uint8, error) {
	if br.nbits == 0 {
		if _, err := io.ReadFull(br.r, br.buf[:]); err != nil {
			return 0, err
		}
		br.cur = br.buf[0]
		br.nbits = 8
	}
	br.nbits--
	return (br.cur >> br.nbits) & 1, nil
}
