package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Junctions need no special privilege, unlike symlinks, and must be treated
// the same way: listed as links, never followed.
func TestJunctionNotFollowed(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "media"), 0o755)
	os.WriteFile(filepath.Join(dir, "media", "big"), make([]byte, 1<<20), 0o644)
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	link := filepath.Join(dir, "docs", "media-link")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, filepath.Join(dir, "media")).CombinedOutput(); err != nil {
		t.Skipf("mklink /J: %v %s", err, out)
	}
	// A junction back up the tree would loop forever if followed.
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(dir, "media", "loop"), dir).CombinedOutput(); err != nil {
		t.Skipf("mklink /J: %v %s", err, out)
	}
	root, _, err := Scan(context.Background(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	l := root.Find("docs/media-link")
	if l == nil || l.Kind != Symlink || len(l.Children) != 0 {
		t.Fatalf("junction should be a childless link: %+v", l)
	}
	if docs := root.Find("docs"); docs.TotalUsage > 64<<10 {
		t.Fatalf("docs total %d includes media through the junction", docs.TotalUsage)
	}
	if root.Find("media/loop").Kind != Symlink {
		t.Fatal("loop junction should be a link")
	}
}
