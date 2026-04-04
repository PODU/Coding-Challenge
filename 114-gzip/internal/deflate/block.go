package deflate

// clItem is one symbol of a dynamic block's run-length-encoded code lengths.
type clItem struct {
	sym       int
	extra     uint32
	extraBits uint
}

// dynamic is a dynamic block's Huffman tables plus its estimated size.
type dynamic struct {
	litEnc  *encoder
	distEnc *encoder
	clEnc   *encoder

	clItems []clItem

	hlit, hdist int // actual counts, not the header's biased values
	numClCodes  int

	headerBits uint64
	symBits    uint64
}

func (c *compressor) flushBlock(final bool) {
	c.litFreq[endOfBlock]++

	data := c.src[c.blockStart:c.blockEnd]

	btype := c.forceType
	if btype < 0 {
		btype = c.chooseBlockType(data)
	}

	switch btype {
	case 0:
		c.emitStored(data, final)
	case 1:
		c.emitHuffman(final, false, nil)
	default:
		c.emitHuffman(final, true, c.buildDynamic())
	}

	c.resetBlock()
}

// chooseBlockType returns whichever BTYPE would encode the pending tokens in
// the fewest bits.
func (c *compressor) chooseBlockType(data []byte) int {
	var fixedSym uint64
	for s, f := range c.litFreq {
		if f > 0 {
			fixedSym += uint64(f) * uint64(fixedLitLengths[s])
		}
	}
	for s, f := range c.distFreq {
		if f > 0 {
			fixedSym += uint64(f) * uint64(fixedDistLengths[s])
		}
	}
	fixedBits := 3 + fixedSym + c.extraBits

	dyn := c.buildDynamic()
	dynBits := 3 + dyn.headerBits + dyn.symBits + c.extraBits

	// A stored block pads to a byte boundary, then has LEN and NLEN.
	pos := c.bw.BitCount() + 3
	pad := (8 - pos%8) % 8
	storedBits := uint64(3) + pad + 32 + uint64(len(data))*8

	best := 1
	bestBits := fixedBits
	if dynBits < bestBits {
		best = 2
		bestBits = dynBits
	}
	if storedBits < bestBits {
		best = 0
	}
	return best
}

func (c *compressor) emitStored(data []byte, final bool) {
	bw := c.bw
	bf := uint32(0)
	if final {
		bf = 1
	}
	bw.WriteBits(bf, 1)
	bw.WriteBits(0, 2) // BTYPE 00
	bw.Align()
	n := uint32(len(data))
	bw.WriteBits(n&0xFFFF, 16)
	bw.WriteBits((^n)&0xFFFF, 16)
	bw.WriteBytes(data)
}

func (c *compressor) emitHuffman(final, isDynamic bool, dyn *dynamic) {
	bw := c.bw
	bf := uint32(0)
	if final {
		bf = 1
	}
	bw.WriteBits(bf, 1)

	var litEnc, distEnc *encoder
	if isDynamic {
		bw.WriteBits(2, 2) // BTYPE 10
		c.writeDynamicHeader(dyn)
		litEnc, distEnc = dyn.litEnc, dyn.distEnc
	} else {
		bw.WriteBits(1, 2) // BTYPE 01
		litEnc, distEnc = fixedLitEnc, fixedDistEnc
	}

	for _, t := range c.tokens {
		if !tokIsMatch(t) {
			litEnc.writeCode(bw, int(tokLiteral(t)))
			continue
		}
		length := tokLength(t)
		dist := tokDist(t)
		lc := lengthCode(length)
		litEnc.writeCode(bw, lc)
		if eb := lengthExtra[lc-257]; eb > 0 {
			bw.WriteBits(uint32(length-lengthBase[lc-257]), eb)
		}
		dc := distCode(dist)
		distEnc.writeCode(bw, dc)
		if eb := distExtra[dc]; eb > 0 {
			bw.WriteBits(uint32(dist-distBase[dc]), eb)
		}
	}
	litEnc.writeCode(bw, endOfBlock)
}

func (c *compressor) writeDynamicHeader(dyn *dynamic) {
	bw := c.bw
	bw.WriteBits(uint32(dyn.hlit-257), 5)
	bw.WriteBits(uint32(dyn.hdist-1), 5)
	bw.WriteBits(uint32(dyn.numClCodes-4), 4)
	for k := 0; k < dyn.numClCodes; k++ {
		bw.WriteBits(uint32(dyn.clEnc.lengths[codeLengthOrder[k]]), 3)
	}
	for _, it := range dyn.clItems {
		dyn.clEnc.writeCode(bw, it.sym)
		if it.extraBits > 0 {
			bw.WriteBits(it.extra, it.extraBits)
		}
	}
}

// buildDynamic builds the Huffman tables for a dynamic block from the current
// symbol counts and works out how many bits the block would take.
func (c *compressor) buildDynamic() *dynamic {
	litLengths := buildLengths(c.litFreq[:], maxBits)
	distLengths := buildLengths(c.distFreq[:], maxBits)

	// Trim unused codes off the end; at least 257 lit/len codes are required.
	hlit := numLitLen
	for hlit > 257 && litLengths[hlit-1] == 0 {
		hlit--
	}
	hdist := numDist
	for hdist > 1 && distLengths[hdist-1] == 0 {
		hdist--
	}

	distUsed := false
	for _, f := range c.distFreq {
		if f > 0 {
			distUsed = true
			break
		}
	}
	if !distUsed {
		// No distances in this block, but send a complete two-code distance
		// tree anyway so strict decoders accept the header.
		distLengths[0] = 1
		distLengths[1] = 1
		hdist = 2
	}

	seq := make([]uint8, 0, hlit+hdist)
	seq = append(seq, litLengths[:hlit]...)
	seq = append(seq, distLengths[:hdist]...)

	clItems := rleEncode(seq)
	var clFreq [numCl]int32
	for _, it := range clItems {
		clFreq[it.sym]++
	}
	clLengths := buildLengths(clFreq[:], maxClBits)
	clEnc := buildEncoder(clLengths)

	numClCodes := numCl
	for numClCodes > 4 && clLengths[codeLengthOrder[numClCodes-1]] == 0 {
		numClCodes--
	}

	dyn := &dynamic{
		litEnc:     buildEncoder(litLengths[:hlit]),
		distEnc:    buildEncoder(distLengths[:hdist]),
		clEnc:      clEnc,
		clItems:    clItems,
		hlit:       hlit,
		hdist:      hdist,
		numClCodes: numClCodes,
	}

	var symBits uint64
	for s, f := range c.litFreq {
		if f > 0 && s < hlit {
			symBits += uint64(f) * uint64(litLengths[s])
		}
	}
	for s, f := range c.distFreq {
		if f > 0 {
			symBits += uint64(f) * uint64(distLengths[s])
		}
	}
	dyn.symBits = symBits

	// HLIT, HDIST and HCLEN, then 3 bits per code-length code, then the
	// encoded lengths themselves.
	headerBits := uint64(14 + 3*numClCodes)
	for _, it := range clItems {
		headerBits += uint64(clLengths[it.sym]) + uint64(it.extraBits)
	}
	dyn.headerBits = headerBits

	return dyn
}

// rleEncode compresses a run of code lengths with symbols 16 (repeat the
// previous length), 17 (3-10 zeros) and 18 (11-138 zeros).
func rleEncode(seq []uint8) []clItem {
	var out []clItem
	i := 0
	for i < len(seq) {
		val := seq[i]
		run := 1
		for i+run < len(seq) && seq[i+run] == val {
			run++
		}
		if val == 0 {
			rem := run
			for rem >= 11 {
				cnt := rem
				if cnt > 138 {
					cnt = 138
				}
				out = append(out, clItem{18, uint32(cnt - 11), 7})
				rem -= cnt
			}
			for rem >= 3 {
				cnt := rem
				if cnt > 10 {
					cnt = 10
				}
				out = append(out, clItem{17, uint32(cnt - 3), 3})
				rem -= cnt
			}
			for ; rem > 0; rem-- {
				out = append(out, clItem{0, 0, 0})
			}
		} else {
			out = append(out, clItem{int(val), 0, 0})
			rem := run - 1
			for rem >= 3 {
				cnt := rem
				if cnt > 6 {
					cnt = 6
				}
				out = append(out, clItem{16, uint32(cnt - 3), 2})
				rem -= cnt
			}
			for ; rem > 0; rem-- {
				out = append(out, clItem{int(val), 0, 0})
			}
		}
		i += run
	}
	return out
}
