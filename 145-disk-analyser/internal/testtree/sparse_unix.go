//go:build !windows

package testtree

import "os"

// makeSparse is truncate -s: extending with ftruncate allocates nothing.
func makeSparse(path string, size int64) (bool, error) {
	f, err := os.Create(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	return true, f.Truncate(size)
}
