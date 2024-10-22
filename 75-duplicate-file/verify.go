package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

// verifyGroup splits files with the same size and hash into sets that are
// byte-for-byte identical. Normally that is one set; this only matters if the
// hash collides.
func verifyGroup(group []FileInfo, errs *[]ScanError) [][]FileInfo {
	var classes [][]FileInfo
	for _, f := range group {
		placed := false
		for ci := range classes {
			eq, err := filesEqual(classes[ci][0], f)
			if err != nil {
				*errs = append(*errs, ScanError{f.Path, err})
				placed = true // drop files we can no longer read
				break
			}
			if eq {
				classes[ci] = append(classes[ci], f)
				placed = true
				break
			}
		}
		if !placed {
			classes = append(classes, []FileInfo{f})
		}
	}
	return classes
}

const compareBufSize = 64 * 1024

// filesEqual compares two files byte by byte. Hardlinks are equal without
// reading anything.
func filesEqual(a, b FileInfo) (bool, error) {
	if a.Dev != 0 && a.Dev == b.Dev && a.Ino == b.Ino {
		return true, nil
	}

	fa, err := os.Open(a.Path)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b.Path)
	if err != nil {
		return false, err
	}
	defer fb.Close()

	ra := bufio.NewReaderSize(fa, compareBufSize)
	rb := bufio.NewReaderSize(fb, compareBufSize)
	bufA := make([]byte, compareBufSize)
	bufB := make([]byte, compareBufSize)

	for {
		na, ea := io.ReadFull(ra, bufA)
		nb, eb := io.ReadFull(rb, bufB)
		if !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false, nil
		}
		endA := ea == io.EOF || ea == io.ErrUnexpectedEOF
		endB := eb == io.EOF || eb == io.ErrUnexpectedEOF
		if endA || endB {
			return endA && endB, nil
		}
		if ea != nil {
			return false, ea
		}
		if eb != nil {
			return false, eb
		}
	}
}
