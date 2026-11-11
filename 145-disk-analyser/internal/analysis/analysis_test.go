package analysis

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"ccdisk/internal/scan"
	"ccdisk/internal/testtree"
)

const MB = testtree.MB

var fixture string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ccdisk-analysis")
	if err != nil {
		panic(err)
	}
	fixture = filepath.Join(dir, "testroot")
	if _, err := testtree.Build(fixture); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func scanFixture(t *testing.T) *scan.Node {
	t.Helper()
	root, _, err := scan.Scan(context.Background(), fixture, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func near(got, want int64) bool { return got >= want && got <= want+want/50+64<<10 }

func TestAgeBands(t *testing.T) {
	root := scanFixture(t)
	now := time.Now()
	if b := BandOf(AgeOf(root.Find("old/archive.bin")), now); b != Old {
		t.Fatalf("archive.bin band %v, want Old", b)
	}
	if b := BandOf(AgeOf(root.Find("media/photo1.bin")), now); b != Fresh {
		t.Fatalf("new file band %v, want Fresh", b)
	}
	if b := BandOf(AgeOf(root.Find("old")), now); b != Old {
		t.Fatal("a directory's age is its newest descendant: old/ should be Old")
	}
	if BandOf(now.Add(-200*24*time.Hour), now) != Recent {
		t.Fatal("200 days should be Recent")
	}
}

func names(root *scan.Node, ns []*scan.Node) []string {
	var out []string
	for _, n := range ns {
		out = append(out, filepath.ToSlash(n.RelPath(root)))
	}
	return out
}

func TestLargestOrder(t *testing.T) {
	root := scanFixture(t)
	got := names(root, Largest(root, Filter{Now: time.Now()}, 4))
	want := []string{"media/video.bin", "projects/webapp/node_modules/lib/big.js", "old/archive.bin", ".cache/app/cache.db"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("largest = %v, want %v", got, want)
		}
	}
}

func TestLargestFilters(t *testing.T) {
	root := scanFixture(t)
	got := names(root, Largest(root, Filter{MinSize: 10 * MB, MinAge: Year, Now: time.Now()}, 0))
	if len(got) != 1 || got[0] != "old/archive.bin" {
		t.Fatalf("10 MB + 1 year filter = %v, want only old/archive.bin", got)
	}
}

func TestLargestScopedToDirectory(t *testing.T) {
	root := scanFixture(t)
	media := root.Find("media")
	for _, n := range Largest(media, Filter{Now: time.Now()}, 0) {
		if !n.Within(media) {
			t.Fatalf("%s is outside media", n.Path())
		}
	}
}

func TestLargestApparentPutsSparseFirst(t *testing.T) {
	root := scanFixture(t)
	if got := Largest(root, Filter{Measure: scan.ApparentSize, Now: time.Now()}, 1); got[0].Name != "sparse.img" {
		t.Fatalf("by apparent size sparse.img is largest, got %s", got[0].Name)
	}
	for _, n := range Largest(root, Filter{Now: time.Now()}, 4) {
		if n.Name == "sparse.img" {
			t.Fatal("sparse.img must not rank by disk usage")
		}
	}
}

func TestReclaimable(t *testing.T) {
	root := scanFixture(t)
	cs := Reclaimable(root)
	got := map[string]Category{}
	var sum int64
	for _, c := range cs {
		got[filepath.ToSlash(c.Node.RelPath(root))] = c.Category
		sum += c.Node.TotalUsage
	}
	want := map[string]Category{
		"projects/webapp/node_modules": Build,
		"projects/api/target":          Build,
		".cache":                       Cache,
	}
	if len(got) != len(want) {
		t.Fatalf("reclaimable = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("reclaimable = %v, want %v", got, want)
		}
	}
	if !near(root.Find("projects/webapp/node_modules").TotalUsage, 21*MB) {
		t.Fatal("node_modules should be about 21 MB")
	}
	if ReclaimableTotal(cs, scan.DiskUsage) != sum {
		t.Fatal("total should be the sum of the marked items")
	}
	if IsReclaimable(root.Find("docs")) || IsReclaimable(root.Find("media")) || !IsReclaimable(root.Find("projects/webapp/node_modules/lib/big.js")) {
		t.Fatal("IsReclaimable")
	}
}

func TestCacheSuffixAboveScanRoot(t *testing.T) {
	// Scanning ~/AppData/Local must still recognise its Temp child, though
	// "AppData" and "Local" are part of the root node's name.
	root := &scan.Node{Name: filepath.Join(string(filepath.Separator)+"home", "AppData", "Local"), Kind: scan.Dir}
	tmp := &scan.Node{Name: "Temp", Kind: scan.Dir, Parent: root}
	root.Children = []*scan.Node{tmp}
	if c, ok := Classify(tmp); !ok || c != Cache {
		t.Fatal("AppData/Local/Temp should be a cache")
	}
	lib := &scan.Node{Name: "Caches", Kind: scan.Dir, Parent: &scan.Node{Name: "Library", Kind: scan.Dir}}
	if _, ok := Classify(lib); !ok {
		t.Fatal("Library/Caches should be a cache")
	}
}

func TestStagingTotals(t *testing.T) {
	root := scanFixture(t)
	s := NewStaging(Policy{Root: root.Path()})
	stage := func(p string) {
		t.Helper()
		if err := s.Add(root.Find(p)); err != nil {
			t.Fatal(err)
		}
	}
	stage("old/archive.bin")
	if !near(s.Total(scan.DiskUsage), 15*MB) {
		t.Fatalf("total %d, want 15 MB", s.Total(scan.DiskUsage))
	}
	stage("projects/webapp/node_modules")
	t36 := s.Total(scan.DiskUsage)
	if !near(t36, 36*MB) {
		t.Fatalf("total %d, want about 36 MB", t36)
	}
	stage("projects/webapp/node_modules/lib/big.js")
	if s.Total(scan.DiskUsage) != t36 {
		t.Fatal("staging something inside a staged directory must not change the total")
	}
	stage("media/video.bin")
	t76 := s.Total(scan.DiskUsage)
	if !near(t76-t36, 40*MB) {
		t.Fatalf("video.bin added %d, want 40 MB", t76-t36)
	}
	stage("media/video-link.bin")
	if s.Total(scan.DiskUsage) != t76 {
		t.Fatal("the hard link's storage must count once")
	}
	s.Remove(root.Find("media/video.bin"))
	if s.Total(scan.DiskUsage) != t76 {
		t.Fatal("video-link.bin alone still holds the 40 MB")
	}
	s.Remove(root.Find("media/video-link.bin"))
	s.Remove(root.Find("projects/webapp/node_modules"))
	if got := s.Total(scan.DiskUsage); !near(got, 15*MB+20*MB) {
		t.Fatalf("after unstaging node_modules (big.js still staged) total %d", got)
	}
	if len(s.Topmost()) != 2 || s.Len() != 2 {
		t.Fatal("Topmost/Len")
	}
}

func TestPolicy(t *testing.T) {
	root := t.TempDir()
	p := DefaultPolicy(root)
	if err := p.Check(filepath.Join(root, "a", "b")); err != nil {
		t.Fatalf("inside root should be allowed: %v", err)
	}
	for _, bad := range []string{root, filepath.Dir(root), filepath.Join(root, "..", "sibling")} {
		if _, ok := IsRefused(p.Check(bad)); !ok {
			t.Errorf("%s should be refused", bad)
		}
	}
	var sys []string
	if runtime.GOOS == "windows" {
		p.Root = `C:\`
		sys = []string{`C:\`, `C:\Windows`, `c:\windows\system32`, `C:\Program Files\x`, `C:\Users`}
	} else {
		p.Root = "/"
		sys = []string{"/", "/usr", "/usr/lib/x", "/bin", "/etc", "/System"}
	}
	for _, s := range sys {
		reason, ok := IsRefused(p.Check(s))
		if !ok || reason == "" {
			t.Errorf("%s should be refused with a reason", s)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if _, ok := IsRefused(p.Check(home)); !ok {
			t.Error("home directory itself should be refused")
		}
	}
}

func TestStagingRefusesOutsideRoot(t *testing.T) {
	root := scanFixture(t)
	s := NewStaging(DefaultPolicy(root.Find("media").Path()))
	if err := s.Add(root.Find("docs/notes.txt")); err == nil {
		t.Fatal("staging outside the scanned root should be refused")
	}
	if err := s.Add(root.Find("media/photo1.bin")); err != nil {
		t.Fatal(err)
	}
}

func TestHumanSize(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", 1023: "1023 B", 40 * MB: "40.0 MB", 1 << 30: "1.0 GB", 150 * MB: "150 MB"} {
		if got := HumanSize(in); got != want {
			t.Errorf("HumanSize(%d) = %q, want %q", in, got, want)
		}
	}
}
