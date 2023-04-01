package huffman

import (
	"bufio"
	"io"
)

// Decompress decodes a HUF1 container from in and writes the original bytes to
// out. Bad input yields ErrBadMagic or ErrCorrupt.
func Decompress(in io.Reader, out io.Writer) error {
	br := bufio.NewReaderSize(in, 64*1024)

	h, err := readHeader(br)
	if err != nil {
		return err
	}

	bw := bufio.NewWriterSize(out, 64*1024)

	if h.symbolCount == 0 {
		return bw.Flush()
	}

	root := buildTree(&h.freq)
	if root == nil {
		return ErrCorrupt
	}

	// With only one distinct symbol the payload carries no information, so
	// just repeat it.
	if root.isLeaf() {
		for i := uint64(0); i < h.symbolCount; i++ {
			if err := bw.WriteByte(root.symbol); err != nil {
				return err
			}
		}
		return bw.Flush()
	}

	bitr := NewBitReader(br)
	for decoded := uint64(0); decoded < h.symbolCount; decoded++ {
		n := root
		for !n.isLeaf() {
			bit, err := bitr.ReadBit()
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return ErrCorrupt
				}
				return err
			}
			if bit == 0 {
				n = n.left
			} else {
				n = n.right
			}
		}
		if err := bw.WriteByte(n.symbol); err != nil {
			return err
		}
	}

	return bw.Flush()
}
