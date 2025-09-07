package bitcask

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	dataExt  = ".cask"
	hintExt  = ".hint"
	lockName = ".lock"
)

func dataFileName(id uint32) string { return fmt.Sprintf("%06d%s", id, dataExt) }

func hintFileName(id uint32) string { return fmt.Sprintf("%06d%s", id, hintExt) }

// activeFile is the single file currently being appended to.
type activeFile struct {
	id     uint32
	f      *os.File
	offset int64
}

// listDataFileIDs returns the ids of all *.cask files in dir, sorted ascending
// (oldest -> newest).
func listDataFileIDs(dir string) ([]uint32, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var ids []uint32
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, dataExt) {
			continue
		}
		base := strings.TrimSuffix(name, dataExt)
		id, err := strconv.ParseUint(base, 10, 32)
		if err != nil {
			continue
		}
		ids = append(ids, uint32(id))
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// acquireLock creates an exclusive advisory lock file for the directory. A
// second writer process fails fast with ErrDatabaseLocked.
func acquireLock(dir string) (*os.File, error) {
	path := filepath.Join(dir, lockName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrDatabaseLocked
		}
		return nil, err
	}
	fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Sync()
	return f, nil
}

func releaseLock(f *os.File) {
	if f == nil {
		return
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
}
