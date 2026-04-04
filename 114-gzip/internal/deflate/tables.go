package deflate

const (
	minMatch   = 3
	maxMatch   = 258
	windowSize = 32768
	windowMask = windowSize - 1

	WindowSize = windowSize

	endOfBlock = 256
	numLitLen  = 286 // literal/length symbols that can actually occur
	numDist    = 30
	maxLitLen  = 288 // the fixed code also defines 286 and 287

	maxBits   = 15 // longest lit/len or distance code
	maxClBits = 7  // longest code-length code
	numCl     = 19
)

// lengthBase / lengthExtra index by length code (257..285) minus 257.
var lengthBase = [29]int{
	3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31,
	35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258,
}

var lengthExtra = [29]uint{
	0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2,
	3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0,
}

// distBase / distExtra index by distance code (0..29).
var distBase = [30]int{
	1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193,
	257, 385, 513, 769, 1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577,
}

var distExtra = [30]uint{
	0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6,
	7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13,
}

// codeLengthOrder is the order code-length code lengths appear in a dynamic
// block header.
var codeLengthOrder = [numCl]int{
	16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15,
}

// lengthCodeTab[L] gives the length code (257..285) for a match length L (3..258).
var lengthCodeTab [maxMatch + 1]int

// distCodeTab[d] gives the distance code (0..29) for distance d (1..32768).
var distCodeTab [windowSize + 1]int

func init() {
	for i := 0; i < 29; i++ {
		base := lengthBase[i]
		n := 1 << lengthExtra[i]
		for l := base; l < base+n && l <= maxMatch; l++ {
			lengthCodeTab[l] = 257 + i
		}
	}
	// 258 also falls inside code 284's range, but it must use 285.
	lengthCodeTab[258] = 285

	for i := 0; i < 30; i++ {
		base := distBase[i]
		n := 1 << distExtra[i]
		for d := base; d < base+n && d <= windowSize; d++ {
			distCodeTab[d] = i
		}
	}
}

func lengthCode(l int) int { return lengthCodeTab[l] }
func distCode(d int) int   { return distCodeTab[d] }

// These are initialised as package vars, not in init, because the init in
// huffman.go needs them to be ready.
var fixedLitLengths = makeFixedLitLengths()
var fixedDistLengths = makeFixedDistLengths()

func makeFixedLitLengths() [maxLitLen]uint8 {
	var a [maxLitLen]uint8
	for i := 0; i < maxLitLen; i++ {
		switch {
		case i < 144:
			a[i] = 8
		case i < 256:
			a[i] = 9
		case i < 280:
			a[i] = 7
		default:
			a[i] = 8
		}
	}
	return a
}

func makeFixedDistLengths() [numDist]uint8 {
	var a [numDist]uint8
	for i := range a {
		a[i] = 5
	}
	return a
}
