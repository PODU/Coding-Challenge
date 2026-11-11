//go:build !windows && !darwin

package cleanup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestXDGTrash(t *testing.T) {
	dir := t.TempDir()
	trash := &XDGTrash{Home: filepath.Join(dir, "Trash"), UID: os.Getuid(),
		Now: func() time.Time { return time.Date(2026, 11, 13, 9, 30, 0, 0, time.UTC) }}
	for i := 0; i < 2; i++ {
		p := filepath.Join(dir, "my file.txt")
		os.WriteFile(p, []byte("x"), 0o644)
		if err := trash.Trash(p); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatal("item should be gone")
		}
	}
	for _, name := range []string{"my file.txt", "my file.txt.2"} {
		if _, err := os.Stat(filepath.Join(trash.Home, "files", name)); err != nil {
			t.Fatalf("files/%s: %v", name, err)
		}
		info, err := os.ReadFile(filepath.Join(trash.Home, "info", name+".trashinfo"))
		if err != nil {
			t.Fatal(err)
		}
		want := "[Trash Info]\nPath=" + strings.ReplaceAll(filepath.Join(dir, "my file.txt"), " ", "%20") +
			"\nDeletionDate=2026-11-13T09:30:00\n"
		if string(info) != want {
			t.Fatalf("trashinfo %q, want %q", info, want)
		}
	}
}

func TestXDGTrashFailureLeavesNoInfo(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	ro := filepath.Join(dir, "readonly")
	os.Mkdir(ro, 0o755)
	os.WriteFile(filepath.Join(ro, "file.bin"), []byte("x"), 0o644)
	os.Chmod(ro, 0o555)
	defer os.Chmod(ro, 0o755)
	trash := &XDGTrash{Home: filepath.Join(dir, "Trash"), UID: os.Getuid(), Now: time.Now}
	if err := trash.Trash(filepath.Join(ro, "file.bin")); err == nil {
		t.Fatal("moving out of a read-only directory should fail")
	}
	if entries, _ := os.ReadDir(filepath.Join(trash.Home, "info")); len(entries) != 0 {
		t.Fatal("a failed move must not leave a .trashinfo behind")
	}
}
