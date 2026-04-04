package deflate

import (
	"errors"
	"sort"

	"gzipcc/internal/bitio"
)

// encoder holds canonical Huffman codes, already bit-reversed for WriteBits.
type encoder struct {
	codes   []uint16
	lengths []uint8
}

func reverseBits(code uint32, length int) uint32 {
	var r uint32
	for i := 0; i < length; i++ {
		r = (r << 1) | (code & 1)
		code >>= 1
	}
	return r
}

func buildEncoder(lengths []uint8) *encoder {
	var blCount [maxBits + 1]int
	maxLen := 0
	for _, l := range lengths {
		blCount[l]++
		if int(l) > maxLen {
			maxLen = int(l)
		}
	}
	blCount[0] = 0

	var nextCode [maxBits + 1]uint32
	var code uint32
	for bits := 1; bits <= maxLen; bits++ {
		code = (code + uint32(blCount[bits-1])) << 1
		nextCode[bits] = code
	}

	enc := &encoder{codes: make([]uint16, len(lengths)), lengths: lengths}
	for sym, l := range lengths {
		if l != 0 {
			c := nextCode[l]
			nextCode[l]++
			enc.codes[sym] = uint16(reverseBits(c, int(l)))
		}
	}
	return enc
}

func (e *encoder) writeCode(w *bitio.Writer, sym int) {
	w.WriteBits(uint32(e.codes[sym]), uint(e.lengths[sym]))
}

// buildLengths returns optimal Huffman code lengths no longer than maxLen,
// using package-merge. Unused symbols get length 0.
func buildLengths(freq []int32, maxLen int) []uint8 {
	lengths := make([]uint8, len(freq))

	type coin struct {
		weight int64
		syms   []int
	}

	var originals []coin
	for i, f := range freq {
		if f > 0 {
			originals = append(originals, coin{int64(f), []int{i}})
		}
	}
	if len(originals) == 0 {
		return lengths
	}
	if len(originals) == 1 {
		lengths[originals[0].syms[0]] = 1
		return lengths
	}
	sort.Slice(originals, func(a, b int) bool {
		return originals[a].weight < originals[b].weight
	})

	mergeSorted := func(a, b []coin) []coin {
		out := make([]coin, 0, len(a)+len(b))
		i, j := 0, 0
		for i < len(a) && j < len(b) {
			if a[i].weight <= b[j].weight {
				out = append(out, a[i])
				i++
			} else {
				out = append(out, b[j])
				j++
			}
		}
		out = append(out, a[i:]...)
		out = append(out, b[j:]...)
		return out
	}

	pkg := func(list []coin) []coin {
		out := make([]coin, 0, len(list)/2)
		for k := 0; k+1 < len(list); k += 2 {
			syms := make([]int, 0, len(list[k].syms)+len(list[k+1].syms))
			syms = append(syms, list[k].syms...)
			syms = append(syms, list[k+1].syms...)
			out = append(out, coin{list[k].weight + list[k+1].weight, syms})
		}
		return out
	}

	list := append([]coin(nil), originals...)
	for i := 1; i < maxLen; i++ {
		list = mergeSorted(originals, pkg(list))
	}

	// Each symbol's code length is the number of times it appears among the
	// cheapest 2n-2 coins.
	need := 2*len(originals) - 2
	for k := 0; k < need && k < len(list); k++ {
		for _, s := range list[k].syms {
			lengths[s]++
		}
	}
	return lengths
}

// decoder is a canonical Huffman decoder (the "puff" algorithm).
type decoder struct {
	count  [maxBits + 1]int
	symbol []int
}

var errBadCode = errors.New("deflate: invalid Huffman code")
var errOversubscribed = errors.New("deflate: oversubscribed Huffman code")

func newDecoder(lengths []uint8) (*decoder, error) {
	d := &decoder{}
	for _, l := range lengths {
		d.count[l]++
	}
	d.count[0] = 0

	left := 1
	for l := 1; l <= maxBits; l++ {
		left <<= 1
		left -= d.count[l]
		if left < 0 {
			return nil, errOversubscribed
		}
	}

	var offs [maxBits + 2]int
	for l := 1; l <= maxBits; l++ {
		offs[l+1] = offs[l] + d.count[l]
	}
	d.symbol = make([]int, len(lengths))
	for sym, l := range lengths {
		if l != 0 {
			d.symbol[offs[l]] = sym
			offs[l]++
		}
	}
	return d, nil
}

func (d *decoder) decode(r *bitio.Reader) (int, error) {
	code, first, index := 0, 0, 0
	for l := 1; l <= maxBits; l++ {
		bit, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		code |= int(bit)
		count := d.count[l]
		if code-first < count {
			return d.symbol[index+code-first], nil
		}
		index += count
		first += count
		first <<= 1
		code <<= 1
	}
	return 0, errBadCode
}

var (
	fixedLitEnc  *encoder
	fixedDistEnc *encoder
	fixedLitDec  *decoder
	fixedDistDec *decoder
)

func init() {
	fixedLitEnc = buildEncoder(fixedLitLengths[:])
	fixedDistEnc = buildEncoder(fixedDistLengths[:])
	fixedLitDec, _ = newDecoder(fixedLitLengths[:])
	fixedDistDec, _ = newDecoder(fixedDistLengths[:])
}
