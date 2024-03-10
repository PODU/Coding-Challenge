package imagefs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type entry struct {
	name string
	typ  byte
	body string
	link string
	mode int64
}

// makeLayer builds an in-memory gzip+tar layer from the given entries.
func makeLayer(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: mode}
		if e.typ == tar.TypeReg {
			hdr.Size = int64(len(e.body))
		}
		if e.typ == tar.TypeSymlink || e.typ == tar.TypeLink {
			hdr.Linkname = e.link
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", e.name, err)
		}
		if e.typ == tar.TypeReg {
			if _, err := io.WriteString(tw, e.body); err != nil {
				t.Fatalf("write body %s: %v", e.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func exists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil
}

func TestExtractBasic(t *testing.T) {
	layer := makeLayer(t, []entry{
		{name: "etc/", typ: tar.TypeDir},
		{name: "etc/hostname", typ: tar.TypeReg, body: "alpine\n"},
		{name: "bin/", typ: tar.TypeDir},
		{name: "bin/sh", typ: tar.TypeReg, body: "#!binary", mode: 0o755},
	})

	dir := t.TempDir()
	if err := Extract(bytes.NewReader(layer), dir); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := readFile(t, dir, "etc/hostname"); got != "alpine\n" {
		t.Errorf("etc/hostname = %q", got)
	}
	if !exists(dir, "bin/sh") {
		t.Errorf("bin/sh missing")
	}
}

func TestStackUpperLayerWinsAndWhiteout(t *testing.T) {
	lower := makeLayer(t, []entry{
		{name: "app/", typ: tar.TypeDir},
		{name: "app/version", typ: tar.TypeReg, body: "v1"},
		{name: "app/old", typ: tar.TypeReg, body: "remove me"},
	})
	upper := makeLayer(t, []entry{
		{name: "app/version", typ: tar.TypeReg, body: "v2"}, // overwrite
		{name: "app/.wh.old", typ: tar.TypeReg, body: ""},   // whiteout app/old
	})

	dir := t.TempDir()
	if err := Stack([]io.Reader{bytes.NewReader(lower), bytes.NewReader(upper)}, dir); err != nil {
		t.Fatalf("Stack: %v", err)
	}
	if got := readFile(t, dir, "app/version"); got != "v2" {
		t.Errorf("app/version = %q; want v2 (upper layer wins)", got)
	}
	if exists(dir, "app/old") {
		t.Errorf("app/old should have been whited out")
	}
}

func TestOpaqueWhiteout(t *testing.T) {
	lower := makeLayer(t, []entry{
		{name: "data/", typ: tar.TypeDir},
		{name: "data/a", typ: tar.TypeReg, body: "a"},
		{name: "data/b", typ: tar.TypeReg, body: "b"},
	})
	upper := makeLayer(t, []entry{
		{name: "data/.wh..wh..opq", typ: tar.TypeReg, body: ""}, // clear data/
		{name: "data/c", typ: tar.TypeReg, body: "c"},
	})

	dir := t.TempDir()
	if err := Stack([]io.Reader{bytes.NewReader(lower), bytes.NewReader(upper)}, dir); err != nil {
		t.Fatalf("Stack: %v", err)
	}
	if exists(dir, "data/a") || exists(dir, "data/b") {
		t.Errorf("opaque whiteout should have removed prior contents of data/")
	}
	if !exists(dir, "data/c") {
		t.Errorf("data/c should remain after opaque whiteout")
	}
}

func TestExtractRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"../evil", "../../etc/passwd", "/abs/evil"} {
		layer := makeLayer(t, []entry{{name: name, typ: tar.TypeReg, body: "x"}})
		dir := t.TempDir()
		if err := Extract(bytes.NewReader(layer), dir); err == nil {
			t.Errorf("Extract(%q) should reject traversal", name)
		}
		// And nothing should have escaped the temp dir's parent.
		if exists(filepath.Dir(dir), "evil") {
			t.Errorf("traversal for %q escaped the destination", name)
		}
	}
}

func TestExtractNonGzipFails(t *testing.T) {
	if err := Extract(bytes.NewReader([]byte("this is not gzip")), t.TempDir()); err == nil {
		t.Errorf("Extract of non-gzip data should fail")
	}
}
