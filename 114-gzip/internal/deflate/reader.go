package deflate

import (
	"errors"
	"io"

	"gzipcc/internal/bitio"
)

var (
	errBadBlockType = errors.New("deflate: reserved block type 11")
	errBadStored    = errors.New("deflate: corrupt stored block length")
	errBadSymbol    = errors.New("deflate: invalid literal/length symbol")
	errBadDist      = errors.New("deflate: invalid distance symbol")
	errDistTooFar   = errors.New("deflate: distance points before start of output")
	errBadClLen     = errors.New("deflate: invalid code-length repeat")
)

// Sink receives inflated output. It must keep at least the last 32 KiB around
// for CopyMatch.
type Sink interface {
	WriteByte(b byte) error
	CopyMatch(dist, length int) error
	Len() int64
}

// Inflate decodes a raw DEFLATE stream into sink and stops after the final
// block, leaving r positioned so the caller can read whatever follows.
func Inflate(r *bitio.Reader, sink Sink) error {
	for {
		bfinal, err := r.ReadBits(1)
		if err != nil {
			return err
		}
		btype, err := r.ReadBits(2)
		if err != nil {
			return err
		}

		switch btype {
		case 0:
			if err := inflateStored(r, sink); err != nil {
				return err
			}
		case 1:
			if err := inflateHuffman(r, sink, fixedLitDec, fixedDistDec); err != nil {
				return err
			}
		case 2:
			litDec, distDec, err := readDynamicTables(r)
			if err != nil {
				return err
			}
			if err := inflateHuffman(r, sink, litDec, distDec); err != nil {
				return err
			}
		default:
			return errBadBlockType
		}

		if bfinal == 1 {
			return nil
		}
	}
}

func inflateStored(r *bitio.Reader, sink Sink) error {
	r.Align()
	b0, err := r.ReadByteAligned()
	if err != nil {
		return unexpected(err)
	}
	b1, err := r.ReadByteAligned()
	if err != nil {
		return unexpected(err)
	}
	n0, err := r.ReadByteAligned()
	if err != nil {
		return unexpected(err)
	}
	n1, err := r.ReadByteAligned()
	if err != nil {
		return unexpected(err)
	}
	length := int(b0) | int(b1)<<8
	nlen := int(n0) | int(n1)<<8
	if length^0xFFFF != nlen {
		return errBadStored
	}
	for i := 0; i < length; i++ {
		b, err := r.ReadByteAligned()
		if err != nil {
			return unexpected(err)
		}
		if err := sink.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}

func inflateHuffman(r *bitio.Reader, sink Sink, litDec, distDec *decoder) error {
	for {
		sym, err := litDec.decode(r)
		if err != nil {
			return err
		}
		switch {
		case sym == endOfBlock:
			return nil
		case sym < 256:
			if err := sink.WriteByte(byte(sym)); err != nil {
				return err
			}
		case sym <= 285:
			lc := sym - 257
			extra, err := r.ReadBits(lengthExtra[lc])
			if err != nil {
				return err
			}
			length := lengthBase[lc] + int(extra)

			dsym, err := distDec.decode(r)
			if err != nil {
				return err
			}
			if dsym >= numDist {
				return errBadDist
			}
			dextra, err := r.ReadBits(distExtra[dsym])
			if err != nil {
				return err
			}
			dist := distBase[dsym] + int(dextra)
			if int64(dist) > sink.Len() {
				return errDistTooFar
			}
			if err := sink.CopyMatch(dist, length); err != nil {
				return err
			}
		default:
			return errBadSymbol
		}
	}
}

func readDynamicTables(r *bitio.Reader) (*decoder, *decoder, error) {
	hlit, err := r.ReadBits(5)
	if err != nil {
		return nil, nil, err
	}
	hdist, err := r.ReadBits(5)
	if err != nil {
		return nil, nil, err
	}
	hclen, err := r.ReadBits(4)
	if err != nil {
		return nil, nil, err
	}
	numLit := int(hlit) + 257
	numD := int(hdist) + 1
	numClCodes := int(hclen) + 4

	var clLengths [numCl]uint8
	for i := 0; i < numClCodes; i++ {
		v, err := r.ReadBits(3)
		if err != nil {
			return nil, nil, err
		}
		clLengths[codeLengthOrder[i]] = uint8(v)
	}
	clDec, err := newDecoder(clLengths[:])
	if err != nil {
		return nil, nil, err
	}

	// Literal/length and distance code lengths are sent as one sequence, and
	// a repeat may run across the boundary between them.
	total := numLit + numD
	lengths := make([]uint8, total)
	for i := 0; i < total; {
		sym, err := clDec.decode(r)
		if err != nil {
			return nil, nil, err
		}
		switch {
		case sym < 16:
			lengths[i] = uint8(sym)
			i++
		case sym == 16:
			if i == 0 {
				return nil, nil, errBadClLen
			}
			rep, err := r.ReadBits(2)
			if err != nil {
				return nil, nil, err
			}
			n := int(rep) + 3
			prev := lengths[i-1]
			for k := 0; k < n; k++ {
				if i >= total {
					return nil, nil, errBadClLen
				}
				lengths[i] = prev
				i++
			}
		case sym == 17:
			rep, err := r.ReadBits(3)
			if err != nil {
				return nil, nil, err
			}
			n := int(rep) + 3
			for k := 0; k < n; k++ {
				if i >= total {
					return nil, nil, errBadClLen
				}
				lengths[i] = 0
				i++
			}
		case sym == 18:
			rep, err := r.ReadBits(7)
			if err != nil {
				return nil, nil, err
			}
			n := int(rep) + 11
			for k := 0; k < n; k++ {
				if i >= total {
					return nil, nil, errBadClLen
				}
				lengths[i] = 0
				i++
			}
		default:
			return nil, nil, errBadClLen
		}
	}

	litDec, err := newDecoder(lengths[:numLit])
	if err != nil {
		return nil, nil, err
	}
	distDec, err := newDecoder(lengths[numLit:])
	if err != nil {
		return nil, nil, err
	}
	return litDec, distDec, nil
}

func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
