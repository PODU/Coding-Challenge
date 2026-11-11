package cleanup

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Uses the real Recycle Bin, so it only runs when asked:
// CCDISK_TEST_RECYCLE=1 go test ./internal/cleanup
func TestRecycleBin(t *testing.T) {
	if os.Getenv("CCDISK_TEST_RECYCLE") == "" {
		t.Skip("set CCDISK_TEST_RECYCLE=1 to test with the real Recycle Bin")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "ccdisk-recycle-test.txt")
	os.WriteFile(p, []byte("restore me"), 0o644)
	if err := (RecycleBin{}).Trash(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(p); !os.IsNotExist(err) {
		t.Fatal("file should be in the Recycle Bin")
	}

	// A file held open without delete sharing cannot be moved; that must be
	// an error, not a silent success.
	locked := filepath.Join(dir, "locked.bin")
	os.WriteFile(locked, []byte("x"), 0o644)
	name, _ := syscall.UTF16PtrFromString(locked)
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)
	if err := (RecycleBin{}).Trash(locked); err == nil {
		if _, serr := os.Lstat(locked); serr == nil {
			t.Fatal("Trash reported success but the locked file is still there")
		}
	}
	if _, err := os.Lstat(locked); err != nil {
		t.Fatal("locked file should still be in place")
	}
}
