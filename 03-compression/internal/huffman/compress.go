package huffman

import (
	"bufio"
	"io"
)

// Stats describes one compression run, for -v output.
type Stats struct {
	InputBytes  uint64
	OutputBytes uint64
	Distinct    int
}

// Ratio returns input size divided by output size, or 0 if nothing was written.
func (s Stats) Ratio() float64 {
	if s.OutputBytes == 0 {
		return 0
	}
	return float64(s.InputBytes) / float64(s.OutputBytes)
}

// Percent returns output size as a percentage of input size.
func (s Stats) Percent() float64 {
	if s.InputBytes == 0 {
		return 0
	}
	return 100 * float64(s.OutputBytes) / float64(s.InputBytes)
}

// Compress writes a HUF1 container for everything read from in. The input is
// held in memory because the frequencies have to be counted before encoding
// can start; that's fine for files of a few MB.
func Compress(in io.Reader, out io.Writer) (Stats, error) {
	data, err := io.ReadAll(in)
	if err != nil {
		return Stats{}, err
	}
	return CompressBytes(data, out)
}

// CompressBytes is Compress for data already in memory.
func CompressBytes(data []byte, out io.Writer) (Stats, error) {
	var freq FreqTable
	for _, b := range data {
		freq[b]++
	}
	symbolCount := uint64(len(data))

	cw := &countingWriter{w: out}
	bw := bufio.NewWriterSize(cw, 64*1024)

	if err := writeHeader(bw, symbolCount, &freq); err != nil {
		return Stats{}, err
	}

	// Empty input is just the header.
	if symbolCount > 0 {
		codes := buildCodes(buildTree(&freq))

		bitw := NewBitWriter(bw)
		for _, b := range data {
			if err := bitw.WriteBits(codes.bits[b], codes.length[b]); err != nil {
				return Stats{}, err
			}
		}
		if err := bitw.Close(); err != nil {
			return Stats{}, err
		}
	}

	if err := bw.Flush(); err != nil {
		return Stats{}, err
	}

	return Stats{
		InputBytes:  symbolCount,
		OutputBytes: cw.n,
		Distinct:    freq.DistinctCount(),
	}, nil
}

type countingWriter struct {
	w io.Writer
	n uint64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += uint64(n)
	return n, err
}
