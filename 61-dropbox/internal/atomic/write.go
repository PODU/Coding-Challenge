// Package atomic writes files via a temp file in the same directory and a
// rename, so a crash never leaves a half-written file behind.
package atomic

import (
	"io"
	"os"
	"path/filepath"
)

// WriteFile writes the bytes produced by fn to path atomically.
func WriteFile(path string, perm os.FileMode, fn func(io.Writer) error) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	if err = fn(tmp); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpName, perm); err != nil {
		return err
	}
	// os.Rename uses MoveFileEx(REPLACE_EXISTING) on Windows, so it overwrites.
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}

// WriteBytes is WriteFile for a byte slice.
func WriteBytes(path string, perm os.FileMode, b []byte) error {
	return WriteFile(path, perm, func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
}
