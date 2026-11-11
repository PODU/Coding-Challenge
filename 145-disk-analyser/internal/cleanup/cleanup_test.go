package cleanup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ccdisk/internal/analysis"
	"ccdisk/internal/scan"
	"ccdisk/internal/testtree"
)

const MB = testtree.MB

// dirTrash moves items into a directory, standing in for the real Trash.
type dirTrash struct {
	dir  string
	fail map[string]bool
}

func (d *dirTrash) Where() string { return "the test trash" }
func (d *dirTrash) Trash(path string) error {
	if d.fail[filepath.Base(path)] {
		return &os.PathError{Op: "rename", Path: path, Err: os.ErrPermission}
	}
	return os.Rename(path, filepath.Join(d.dir, filepath.Base(path)))
}

func setup(t *testing.T) (*scan.Node, *dirTrash, analysis.Policy) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "testroot")
	if _, err := testtree.Build(root); err != nil {
		t.Fatal(err)
	}
	trash := filepath.Join(dir, "trash")
	os.Mkdir(trash, 0o755)
	n, _, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return n, &dirTrash{dir: trash, fail: map[string]bool{}}, analysis.DefaultPolicy(n.Path())
}

func countFiles(root string) int {
	n := 0
	filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			n++
		}
		return nil
	})
	return n
}

func TestMovesOnlyReviewedItems(t *testing.T) {
	root, trash, pol := setup(t)
	before := countFiles(root.Path())
	items := []*scan.Node{root.Find("old/archive.bin"), root.Find(".cache/app/cache.db")}
	r := Execute(items, pol, trash)
	if r.Count(Moved) != 2 || r.Count(Skipped)+r.Count(Failed) != 0 {
		t.Fatalf("outcomes %+v", r.Outcomes)
	}
	if r.Moved < 25*MB || r.Moved > 25*MB+64<<10 {
		t.Fatalf("moved %d, want 25 MB", r.Moved)
	}
	if got := countFiles(root.Path()); got != before-2 {
		t.Fatalf("files %d, want %d", got, before-2)
	}
	if _, err := os.Stat(filepath.Join(trash.dir, "archive.bin")); err != nil {
		t.Fatal("archive.bin should be recoverable from the trash")
	}
}

func TestSkipsVanishedAndChangedItems(t *testing.T) {
	root, trash, pol := setup(t)
	archive, notes, photo := root.Find("old/archive.bin"), root.Find("docs/notes.txt"), root.Find("media/photo1.bin")
	// What the challenge does from another terminal before confirming.
	os.Remove(archive.Path())
	os.Remove(notes.Path())
	os.Mkdir(notes.Path(), 0o755)

	r := Execute([]*scan.Node{archive, notes, photo}, pol, trash)
	want := map[string]string{"archive.bin": "no longer exists", "notes.txt": "changed from a file to a directory"}
	for _, o := range r.Outcomes {
		if reason, ok := want[o.Node.Name]; ok {
			if o.Status != Skipped || !strings.Contains(o.Reason, reason) {
				t.Errorf("%s: %v %q, want skipped %q", o.Node.Name, o.Status, o.Reason, reason)
			}
		} else if o.Status != Moved {
			t.Errorf("%s: %v %q", o.Node.Name, o.Status, o.Reason)
		}
	}
	if fi, err := os.Stat(notes.Path()); err != nil || !fi.IsDir() {
		t.Fatal("the new notes.txt directory must be left alone")
	}
	if r.Moved > 8*MB+64<<10 {
		t.Fatalf("skipped items must not count as reclaimed: %d", r.Moved)
	}
}

func TestSkipsReplacedFile(t *testing.T) {
	root, trash, pol := setup(t)
	n := root.Find("docs/report.pdf")
	tmp := n.Path() + ".new"
	os.WriteFile(tmp, []byte("different"), 0o644)
	os.Remove(n.Path())
	os.Rename(tmp, n.Path())
	fi, _ := os.Lstat(n.Path())
	if scan.Identify(n.Path(), fi) == n.ID {
		t.Skip("filesystem reused the file ID")
	}
	r := Execute([]*scan.Node{n}, pol, trash)
	if r.Outcomes[0].Status != Skipped || !strings.Contains(r.Outcomes[0].Reason, "replaced") {
		t.Fatalf("%+v", r.Outcomes[0])
	}
}

func TestFailureIsNotCountedAsReclaimed(t *testing.T) {
	root, trash, pol := setup(t)
	trash.fail["photo2.bin"] = true
	r := Execute([]*scan.Node{root.Find("media/photo2.bin"), root.Find("media/photo1.bin")}, pol, trash)
	if r.Count(Failed) != 1 || r.Count(Moved) != 1 {
		t.Fatalf("%+v", r.Outcomes)
	}
	if r.Moved > 8*MB+64<<10 {
		t.Fatalf("failed item counted: moved %d", r.Moved)
	}
	if _, err := os.Stat(root.Find("media/photo2.bin").Path()); err != nil {
		t.Fatal("failed item must stay in place")
	}
}

type liar struct{}

func (liar) Where() string           { return "nowhere" }
func (liar) Trash(path string) error { return nil }

func TestItemStillPresentIsFailure(t *testing.T) {
	root, _, pol := setup(t)
	r := Execute([]*scan.Node{root.Find("media/photo2.bin")}, pol, liar{})
	if r.Outcomes[0].Status != Failed || r.Moved != 0 {
		t.Fatalf("a trasher that reports success without moving: %+v", r.Outcomes[0])
	}
}

func TestRefusedPathsSkipped(t *testing.T) {
	root, trash, _ := setup(t)
	pol := analysis.DefaultPolicy(root.Find("media").Path())
	r := Execute([]*scan.Node{root.Find("docs/report.pdf")}, pol, trash)
	if r.Outcomes[0].Status != Skipped || !strings.Contains(r.Outcomes[0].Reason, "outside the scanned root") {
		t.Fatalf("%+v", r.Outcomes[0])
	}
	if _, err := os.Stat(root.Find("docs/report.pdf").Path()); err != nil {
		t.Fatal("refused item must stay")
	}
}

func TestNestedItemsHandledOnce(t *testing.T) {
	root, trash, pol := setup(t)
	nm := root.Find("projects/webapp/node_modules")
	r := Execute([]*scan.Node{nm, root.Find("projects/webapp/node_modules/lib/big.js")}, pol, trash)
	if len(r.Outcomes) != 1 || r.Outcomes[0].Node != nm || r.Outcomes[0].Status != Moved {
		t.Fatalf("%+v", r.Outcomes)
	}
}

func TestReportsFreeSpace(t *testing.T) {
	root, trash, pol := setup(t)
	r := Execute([]*scan.Node{root.Find("media/photo2.bin")}, pol, trash)
	if r.FreeErr != nil || r.FreeBefore <= 0 || r.FreeAfter <= 0 {
		t.Fatalf("free before %d after %d err %v", r.FreeBefore, r.FreeAfter, r.FreeErr)
	}
}

func TestErrText(t *testing.T) {
	if errText(&os.PathError{Op: "x", Path: "/p", Err: os.ErrPermission}) != os.ErrPermission.Error() {
		t.Fatal("PathError")
	}
	if errText(errors.New("plain")) != "plain" {
		t.Fatal("plain")
	}
}
