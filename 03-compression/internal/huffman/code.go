package huffman

// CodeTable holds the Huffman code for each byte value, right-aligned in bits.
// A length of 0 means the byte never appeared in the input.
type CodeTable struct {
	bits   [256]uint64
	length [256]uint8
}

// buildCodes assigns each leaf its path from the root (left 0, right 1).
//
// A tree with a single leaf would give that symbol a zero-length code, so it
// gets the 1-bit code "0" instead.
func buildCodes(root *node) CodeTable {
	var ct CodeTable
	if root == nil {
		return ct
	}
	if root.isLeaf() {
		ct.bits[root.symbol] = 0
		ct.length[root.symbol] = 1
		return ct
	}
	var walk func(n *node, bits uint64, depth uint8)
	walk = func(n *node, bits uint64, depth uint8) {
		if n.isLeaf() {
			ct.bits[n.symbol] = bits
			ct.length[n.symbol] = depth
			return
		}
		walk(n.left, bits<<1, depth+1)
		walk(n.right, (bits<<1)|1, depth+1)
	}
	walk(root, 0, 0)
	return ct
}
