package scan

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"ccdisk/internal/testtree"
)

const MB = testtree.MB

var (
	fixture string
	built   testtree.Result
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ccdisk-scan")
	if err != nil {
		panic(err)
	}
	fixture = filepath.Join(dir, "testroot")
	if built, err = testtree.Build(fixture); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func scanFixture(t *testing.T) *Node {
	t.Helper()
	root, warns, err := Scan(context.Background(), fixture, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) > 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	return root
}

func countFind(t *testing.T, dir string) int {
	n := 0
	filepath.WalkDir(dir, func(string, fs.DirEntry, error) error { n++; return nil })
	return n
}

func TestItemCountMatchesFind(t *testing.T) {
	root := scanFixture(t)
	if got, want := root.Items+1, countFind(t, fixture); got != want {
		t.Fatalf("items %d, find-equivalent %d", got, want)
	}
}

func TestUnreadableDirectoryWarnsAndContinues(t *testing.T) {
	s := newScanner(Options{})
	denied := filepath.Join(fixture, "locked")
	s.readDir = func(p string) ([]os.DirEntry, error) {
		if p == denied {
			return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrPermission}
		}
		return os.ReadDir(p)
	}
	root, warns, err := s.scan(context.Background(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || warns[0].Path != denied || !errors.Is(warns[0].Err, fs.ErrPermission) {
		t.Fatalf("want one permission warning for locked, got %v", warns)
	}
	locked := root.Find("locked")
	if locked == nil || locked.Err == nil || len(locked.Children) != 0 {
		t.Fatalf("locked should be present, flagged and empty: %+v", locked)
	}
	if root.Find("media/photo1.bin") == nil {
		t.Fatal("scan should carry on past the unreadable directory")
	}
	// find counts locked but cannot see hidden.txt.
	if got, want := root.Items+1, countFind(t, fixture)-1; got != want {
		t.Fatalf("items %d, want %d", got, want)
	}
}

func hashTree(t *testing.T) string {
	var lines []string
	filepath.WalkDir(fixture, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		fi, _ := os.Lstat(p)
		lines = append(lines, fmt.Sprintf("%x %s %d", sha256.Sum256(b), p, fi.ModTime().UnixNano()))
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func TestScanIsReadOnly(t *testing.T) {
	before := hashTree(t)
	scanFixture(t)
	if after := hashTree(t); after != before {
		t.Fatal("scan changed file contents or mtimes")
	}
}

func TestSparseFile(t *testing.T) {
	if !built.Sparse {
		t.Skip("filesystem does not support sparse files")
	}
	n := scanFixture(t).Find("sparse.img")
	if n.Apparent != 1<<30 {
		t.Fatalf("apparent %d, want 1 GiB", n.Apparent)
	}
	if n.Usage > 1*MB {
		t.Fatalf("disk usage %d, want near zero", n.Usage)
	}
}

func TestSmallFileTakesABlock(t *testing.T) {
	n := scanFixture(t).Find("docs/notes.txt")
	if n.Apparent != 11 {
		t.Fatalf("apparent %d", n.Apparent)
	}
	// Some filesystems store tiny files inline (0 blocks); otherwise at least
	// one whole block.
	if n.Usage != 0 && n.Usage < 512 {
		t.Fatalf("usage %d is less than a block", n.Usage)
	}
}

func TestHardLinkCountedOnce(t *testing.T) {
	root := scanFixture(t)
	media := root.Find("media")
	video, link := root.Find("media/video.bin"), root.Find("media/video-link.bin")
	if video.CountedUsage < 40*MB || link.CountedUsage != 0 {
		t.Fatalf("video.bin should own the storage: video %d, link %d", video.CountedUsage, link.CountedUsage)
	}
	if media.TotalUsage < 50*MB || media.TotalUsage > 51*MB {
		t.Fatalf("media total %d, want about 50 MB", media.TotalUsage)
	}
	if media.TotalApparent != 50*MB+media.Apparent {
		t.Fatalf("media apparent %d, want 50 MB", media.TotalApparent)
	}
	if media.Files != 4 {
		t.Fatalf("media files %d, want 4 (the hard link is a separate name)", media.Files)
	}
}

func TestSymlinkNotFollowed(t *testing.T) {
	if !built.Symlink {
		t.Skip("cannot create symlinks here (Windows needs developer mode)")
	}
	root := scanFixture(t)
	l := root.Find("docs/media-link")
	if l == nil || l.Kind != Symlink || len(l.Children) != 0 {
		t.Fatalf("media-link should be a childless link: %+v", l)
	}
	if docs := root.Find("docs"); docs.TotalUsage > 6*MB {
		t.Fatalf("docs total %d includes media through the link", docs.TotalUsage)
	}
}

func TestChildSharesSumToParent(t *testing.T) {
	root := scanFixture(t)
	var check func(n *Node)
	check = func(n *Node) {
		if n.Kind != Dir {
			return
		}
		sum := n.Usage
		for _, c := range n.Children {
			sum += c.TotalUsage
			check(c)
		}
		if sum != n.TotalUsage {
			t.Errorf("%s: children sum %d, total %d", n.Path(), sum, n.TotalUsage)
		}
	}
	check(root)
}

func TestDirectoryAgeIsNewestDescendant(t *testing.T) {
	dir := t.TempDir()
	old := time.Date(2018, 1, 1, 0, 0, 0, 0, time.Local)
	os.Mkdir(filepath.Join(dir, "oldproject"), 0o755)
	os.WriteFile(filepath.Join(dir, "oldproject", "a.txt"), []byte("old\n"), 0o644)
	os.Chtimes(filepath.Join(dir, "oldproject", "a.txt"), old, old)
	os.Chtimes(filepath.Join(dir, "oldproject"), old, old)
	root, _, _ := Scan(context.Background(), dir, Options{})
	if got := root.Find("oldproject").Newest; !got.Equal(old) {
		t.Fatalf("newest %v, want 2018", got)
	}
	os.WriteFile(filepath.Join(dir, "oldproject", "b.txt"), []byte("new\n"), 0o644)
	root, _, _ = Scan(context.Background(), dir, Options{})
	if got := root.Find("oldproject").Newest; time.Since(got) > time.Hour {
		t.Fatalf("newest %v, want now", got)
	}
}

func TestRescanPicksUpChanges(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a"), 0o755)
	os.WriteFile(filepath.Join(dir, "a", "x"), make([]byte, 8192), 0o644)
	root, _, _ := Scan(context.Background(), dir, Options{})
	a := root.Find("a")
	os.Remove(filepath.Join(dir, "a", "x"))
	os.WriteFile(filepath.Join(dir, "a", "y"), make([]byte, 100), 0o644)
	if _, err := Rescan(context.Background(), a, Options{}); err != nil {
		t.Fatal(err)
	}
	Recompute(root)
	if root.Find("a/x") != nil || root.Find("a/y") == nil || root.Find("a") != a {
		t.Fatal("rescan should replace children in place")
	}
	if root.TotalApparent != a.TotalApparent+root.Apparent || a.Files != 1 {
		t.Fatalf("totals not updated: root %d a %d files %d", root.TotalApparent, a.TotalApparent, a.Files)
	}
}

func TestProgressCounts(t *testing.T) {
	var p Progress
	root, _, _ := Scan(context.Background(), fixture, Options{Progress: &p, Workers: 2})
	if int(p.Items.Load()) != root.Items || p.Bytes.Load() < root.TotalUsage-root.Usage {
		t.Fatalf("progress items %d bytes %d vs %d %d", p.Items.Load(), p.Bytes.Load(), root.Items, root.TotalUsage)
	}
}

func TestPaths(t *testing.T) {
	root := scanFixture(t)
	n := root.Find("media/video.bin")
	if n.Path() != filepath.Join(fixture, "media", "video.bin") {
		t.Fatal(n.Path())
	}
	if n.DisplayPath() != filepath.Join("testroot", "media", "video.bin") {
		t.Fatal(n.DisplayPath())
	}
	if n.RelPath(root.Find("media")) != "video.bin" || !n.Within(root) || n.Within(root.Find("docs")) {
		t.Fatal("RelPath/Within")
	}
}

func TestFreeSpace(t *testing.T) {
	free, total, err := FreeSpace(fixture)
	if err != nil || free <= 0 || total < free {
		t.Fatalf("free %d total %d err %v", free, total, err)
	}
}
