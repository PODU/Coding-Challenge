package main

import (
	"bufio"
	"bytes"
	"io"
)

const chunkSize = 64 * 1024

// lastLinesStart returns the offset where the last n lines begin. It scans
// backwards from the end, so the cost depends on the output, not the file.
//
// A newline in the very last byte ends the final line rather than starting an
// empty one, so it isn't counted.
func lastLinesStart(r io.ReaderAt, size, n int64) (int64, error) {
	if n <= 0 {
		return size, nil
	}
	buf := make([]byte, chunkSize)
	pos := size
	for pos > 0 {
		readLen := min(int64(chunkSize), pos)
		pos -= readLen
		b := buf[:readLen]
		if _, err := r.ReadAt(b, pos); err != nil && err != io.EOF {
			return 0, err
		}
		for i := len(b) - 1; i >= 0; i-- {
			if b[i] != '\n' || pos+int64(i) == size-1 {
				continue
			}
			n--
			if n == 0 {
				return pos + int64(i) + 1, nil
			}
		}
	}
	return 0, nil
}

func lastBytesStart(size, n int64) int64 {
	return max(0, size-n)
}

func copyRange(r io.ReaderAt, start, end int64, w io.Writer) error {
	_, err := io.Copy(w, io.NewSectionReader(r, start, end-start))
	return err
}

// streamLastLines writes the last n lines of a stream that can't seek,
// keeping only n lines in memory. It returns how many bytes it read.
func streamLastLines(r io.Reader, n int64, w io.Writer) (int64, error) {
	br := bufio.NewReaderSize(r, chunkSize)
	var (
		ring  [][]byte
		next  int // slot to overwrite once the ring is full
		total int64
	)
	for {
		line, err := br.ReadBytes('\n')
		total += int64(len(line))
		if len(line) > 0 && n > 0 {
			if int64(len(ring)) < n {
				ring = append(ring, line)
			} else {
				ring[next] = line
				next = (next + 1) % len(ring)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return total, err
		}
	}
	for i := range ring {
		if _, err := w.Write(ring[(next+i)%len(ring)]); err != nil {
			return total, err
		}
	}
	return total, nil
}

// streamLastBytes is streamLastLines for bytes. It trims its buffer only once
// it grows past 2n so it isn't copying on every read.
func streamLastBytes(r io.Reader, n int64, w io.Writer) (int64, error) {
	var (
		keep  []byte
		total int64
	)
	buf := make([]byte, chunkSize)
	for {
		m, err := r.Read(buf)
		total += int64(m)
		if n > 0 && m > 0 {
			keep = append(keep, buf[:m]...)
			if int64(len(keep)) > 2*n+chunkSize {
				keep = append(keep[:0], keep[int64(len(keep))-n:]...)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return total, err
		}
	}
	if int64(len(keep)) > n {
		keep = keep[int64(len(keep))-n:]
	}
	_, err := w.Write(keep)
	return total, err
}

// fromLine writes r from 1-based line k onwards and returns how many bytes it
// read.
func fromLine(r io.Reader, k int64, w io.Writer) (int64, error) {
	var total int64
	skip := k - 1
	buf := make([]byte, chunkSize)
	for skip > 0 {
		m, err := r.Read(buf)
		total += int64(m)
		b := buf[:m]
		for skip > 0 {
			i := bytes.IndexByte(b, '\n')
			if i < 0 {
				b = nil
				break
			}
			b = b[i+1:]
			skip--
		}
		if len(b) > 0 {
			if _, werr := w.Write(b); werr != nil {
				return total, werr
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
	c, err := io.Copy(w, r)
	return total + c, err
}

// fromByte writes r from 1-based byte k onwards and returns how many bytes it
// read.
func fromByte(r io.Reader, k int64, w io.Writer) (int64, error) {
	skipped, err := io.CopyN(io.Discard, r, max(0, k-1))
	if err == io.EOF {
		return skipped, nil
	}
	if err != nil {
		return skipped, err
	}
	c, err := io.Copy(w, r)
	return skipped + c, err
}
