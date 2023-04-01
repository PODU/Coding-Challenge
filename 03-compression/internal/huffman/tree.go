package huffman

import "container/heap"

type node struct {
	symbol byte // leaves only
	freq   uint64
	left   *node
	right  *node
	order  int // insertion order, used as the final tie-break
}

func (n *node) isLeaf() bool { return n.left == nil && n.right == nil }

// nodeHeap is a min-heap ordered by freq, then leaves first, then symbol,
// then insertion order.
type nodeHeap []*node

func (h nodeHeap) Len() int { return len(h) }

func (h nodeHeap) Less(i, j int) bool {
	a, b := h[i], h[j]
	if a.freq != b.freq {
		return a.freq < b.freq
	}
	// The encoder and decoder must build exactly the same tree, so ties have
	// to be broken deterministically.
	ai, bi := a.isLeaf(), b.isLeaf()
	if ai != bi {
		return ai
	}
	if ai && bi && a.symbol != b.symbol {
		return a.symbol < b.symbol
	}
	return a.order < b.order
}

func (h nodeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *nodeHeap) Push(x any) { *h = append(*h, x.(*node)) }

func (h *nodeHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

// buildTree returns the root of the Huffman tree for t, or nil if t is empty.
// With one distinct symbol the root is a single leaf (see buildCodes).
func buildTree(t *FreqTable) *node {
	h := &nodeHeap{}
	order := 0
	for sym := 0; sym < 256; sym++ {
		if t[sym] > 0 {
			heap.Push(h, &node{symbol: byte(sym), freq: t[sym], order: order})
			order++
		}
	}
	if h.Len() == 0 {
		return nil
	}
	for h.Len() > 1 {
		a := heap.Pop(h).(*node)
		b := heap.Pop(h).(*node)
		heap.Push(h, &node{
			freq:  a.freq + b.freq,
			left:  a,
			right: b,
			order: order,
		})
		order++
	}
	return heap.Pop(h).(*node)
}
