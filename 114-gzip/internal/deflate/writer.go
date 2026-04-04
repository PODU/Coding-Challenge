package deflate

import (
	"io"

	"gzipcc/internal/bitio"
)

// A token is a literal byte, or matchFlag | (length-3)<<15 | (dist-1).
const matchFlag = 1 << 30

func litToken(b byte) uint32 { return uint32(b) }
func matchToken(length, dist int) uint32 {
	return matchFlag | uint32(length-3)<<15 | uint32(dist-1)
}
func tokIsMatch(t uint32) bool { return t&matchFlag != 0 }
func tokLiteral(t uint32) byte { return byte(t) }
func tokLength(t uint32) int   { return int((t>>15)&0xFF) + 3 }
func tokDist(t uint32) int     { return int(t&0x7FFF) + 1 }

// levelConfig holds zlib's match-finder tuning knobs for one level.
type levelConfig struct {
	good  int
	lazy  int
	nice  int
	chain int
}

var levelConfigs = [10]levelConfig{
	{0, 0, 0, 0},         // 0 - stored only
	{4, 4, 8, 4},         // 1
	{4, 5, 16, 8},        // 2
	{4, 6, 32, 32},       // 3
	{4, 4, 16, 16},       // 4
	{8, 16, 32, 32},      // 5
	{8, 16, 128, 128},    // 6
	{8, 32, 128, 256},    // 7
	{32, 128, 258, 1024}, // 8
	{32, 258, 258, 4096}, // 9
}

// blockBudget keeps each block small enough to fall back to a stored block,
// whose LEN is 16 bits, even if the last token is a 258-byte match.
const blockBudget = 1<<16 - 1024

const hashBits = 15
const hashSize = 1 << hashBits

func hashAt(src []byte, i int) uint32 {
	h := uint32(src[i])<<16 | uint32(src[i+1])<<8 | uint32(src[i+2])
	return (h * 506832829) >> (32 - hashBits) & (hashSize - 1)
}

type compressor struct {
	src   []byte
	bw    *bitio.Writer
	level int
	cfg   levelConfig

	head []int32
	prev []int32

	tokens    []uint32
	litFreq   [maxLitLen]int32
	distFreq  [numDist]int32
	extraBits uint64

	blockStart int // src range covered by the pending tokens
	blockEnd   int

	forceType int // tests set this to force a block type; -1 picks the cheapest
}

// Compress writes src to w as a raw DEFLATE stream. Out-of-range levels mean 6.
func Compress(w io.Writer, src []byte, level int) error {
	if level < 0 || level > 9 {
		level = 6
	}
	bw := bitio.NewWriter(w)
	c := &compressor{
		src:       src,
		bw:        bw,
		level:     level,
		cfg:       levelConfigs[level],
		forceType: -1,
	}
	c.run()
	bw.Align()
	return bw.Flush()
}

func (c *compressor) run() {
	if c.level == 0 || len(c.src) == 0 {
		c.storedOnly()
		return
	}
	c.head = make([]int32, hashSize)
	for i := range c.head {
		c.head[i] = -1
	}
	c.prev = make([]int32, windowSize)
	c.deflate()
}

// storedOnly writes src as stored blocks. Empty input still needs one block.
func (c *compressor) storedOnly() {
	if len(c.src) == 0 {
		c.emitStored(nil, true)
		return
	}
	for off := 0; off < len(c.src); {
		end := min(off+0xFFFF, len(c.src))
		c.emitStored(c.src[off:end], end == len(c.src))
		off = end
	}
}

func (c *compressor) emitLiteral(b byte) {
	c.tokens = append(c.tokens, litToken(b))
	c.litFreq[b]++
	c.blockEnd++
}

func (c *compressor) emitMatch(length, dist int) {
	c.tokens = append(c.tokens, matchToken(length, dist))
	lc := lengthCode(length)
	dc := distCode(dist)
	c.litFreq[lc]++
	c.distFreq[dc]++
	c.extraBits += uint64(lengthExtra[lc-257]) + uint64(distExtra[dc])
	c.blockEnd += length
}

func (c *compressor) deflate() {
	src := c.src
	n := len(src)

	insert := func(pos int) int32 {
		if pos+minMatch > n {
			return -1
		}
		h := hashAt(src, pos)
		head := c.head[h]
		c.prev[pos&windowMask] = head
		c.head[h] = int32(pos)
		return head
	}

	i := 0
	haveMatch := false
	matchLen, matchDist := 0, 0

	for i < n {
		curLen, curDist := 0, 0
		head := insert(i)
		if head >= 0 {
			l, d := c.longestMatch(i, int(head), matchLen)
			if l >= minMatch {
				curLen, curDist = l, d
			}
		}

		if haveMatch {
			if curLen > matchLen {
				// Lazy matching: a longer match starts here, so the previous
				// position becomes a literal.
				c.emitLiteral(src[i-1])
				matchLen, matchDist = curLen, curDist
				i++
			} else {
				c.emitMatch(matchLen, matchDist)
				end := (i - 1) + matchLen
				for j := i + 1; j < end; j++ {
					insert(j)
				}
				i = end
				haveMatch = false
			}
		} else {
			if curLen >= minMatch {
				matchLen, matchDist = curLen, curDist
				haveMatch = true
			} else {
				c.emitLiteral(src[i])
			}
			i++
		}

		if c.blockEnd-c.blockStart >= blockBudget {
			c.flushBlock(false)
		}
	}

	if haveMatch {
		c.emitMatch(matchLen, matchDist)
	}
	c.flushBlock(true)
}

// longestMatch walks the hash chain from start looking for the longest match
// at pos. If prevLen is already a good match, the chain is searched less deeply.
func (c *compressor) longestMatch(pos, start, prevLen int) (int, int) {
	src := c.src
	n := len(src)
	maxLen := maxMatch
	if n-pos < maxLen {
		maxLen = n - pos
	}
	if maxLen < minMatch {
		return 0, 0
	}

	nice := c.cfg.nice
	if nice > maxLen {
		nice = maxLen
	}
	chain := c.cfg.chain
	if prevLen >= c.cfg.good {
		chain >>= 2
	}

	limit := pos - windowSize
	if limit < 0 {
		limit = 0
	}

	bestLen := minMatch - 1
	bestPos := -1
	cur := start
	for chain > 0 && cur >= limit {
		if src[cur+bestLen] == src[pos+bestLen] && src[cur] == src[pos] {
			l := matchLength(src, cur, pos, maxLen)
			if l > bestLen {
				bestLen = l
				bestPos = cur
				if l >= nice {
					break
				}
			}
		}
		cur = int(c.prev[cur&windowMask])
		chain--
	}

	if bestLen >= minMatch {
		return bestLen, pos - bestPos
	}
	return 0, 0
}

func matchLength(src []byte, a, b, maxLen int) int {
	l := 0
	for l < maxLen && src[a+l] == src[b+l] {
		l++
	}
	return l
}

func (c *compressor) resetBlock() {
	c.tokens = c.tokens[:0]
	for i := range c.litFreq {
		c.litFreq[i] = 0
	}
	for i := range c.distFreq {
		c.distFreq[i] = 0
	}
	c.extraBits = 0
	c.blockStart = c.blockEnd
}
