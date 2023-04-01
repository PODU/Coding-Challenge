package huffman

import (
	"bufio"
	"io"
)

// FreqTable counts occurrences of each byte value.
type FreqTable [256]uint64

// CountFrequencies counts byte values in r and returns the total number of
// bytes read.
func CountFrequencies(r io.Reader) (FreqTable, uint64, error) {
	var table FreqTable
	var total uint64

	br := bufio.NewReaderSize(r, 64*1024)
	buf := make([]byte, 64*1024)
	for {
		n, err := br.Read(buf)
		for _, b := range buf[:n] {
			table[b]++
		}
		total += uint64(n)
		if err != nil {
			if err == io.EOF {
				break
			}
			return table, total, err
		}
	}
	return table, total, nil
}

// DistinctCount returns how many byte values occur at least once.
func (t *FreqTable) DistinctCount() int {
	n := 0
	for _, c := range t {
		if c > 0 {
			n++
		}
	}
	return n
}
