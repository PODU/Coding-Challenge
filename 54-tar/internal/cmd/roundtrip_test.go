package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildTree creates a representative source tree and returns its root.
func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "hello.txt"), "hello world\n", 0o644)
	mustWrite(t, filepath.Join(root, "nested", "deep", "data.bin"), strings.Repeat("\x00\x01\x02\x03", 300), 0o600)
	mustWrite(t, filepath.Join(root, "empty.txt"), "", 0o644)
	if err := os.MkdirAll(filepath.Join(root, "emptydir"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Long name: a directory whose path exceeds 100 bytes, forcing GNU 'L'.
	longDir := filepath.Join(root, strings.Repeat("d", 120))
	mustWrite(t, filepath.Join(longDir, "x.txt"), "long path content", 0o644)
	return root
}

func mustWrite(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestCreateExtractRoundTrip(t *testing.T) {
	src := buildTree(t)

	var arch bytes.Buffer
	if err := Create(&arch, []string{src}, false, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	dest := t.TempDir()
	if err := Extract(bytes.NewReader(arch.Bytes()), dest, false, nil); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// The archive stored absolute-ish names rooted at src's cleaned path. Walk
	// the source and compare each file's content under dest using the archive
	// name mapping.
	srcName := toArchiveName(filepath.Clean(src), true)
	_ = srcName
	compareTrees(t, src, dest)
}

// compareTrees asserts every regular file under src exists with identical
// content under dest (matched by the archive name relative layout).
func compareTrees(t *testing.T, src, dest string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		archName := toArchiveName(path, info.IsDir())
		target := filepath.Join(dest, filepath.FromSlash(archName))
		di, err := os.Lstat(target)
		if err != nil {
			t.Errorf("missing in dest: %s (%v)", archName, err)
			return nil
		}
		if info.IsDir() {
			if !di.IsDir() {
				t.Errorf("%s should be a dir in dest", archName)
			}
			return nil
		}
		if info.Mode().IsRegular() {
			want, _ := os.ReadFile(path)
			got, err := os.ReadFile(target)
			if err != nil {
				t.Errorf("read %s: %v", target, err)
				return nil
			}
			if !bytes.Equal(want, got) {
				t.Errorf("content mismatch for %s", archName)
			}
			// mode (perm bits) on non-Windows
			if runtime.GOOS != "windows" {
				if info.Mode().Perm() != di.Mode().Perm() {
					t.Errorf("mode mismatch for %s: %v != %v", archName, info.Mode().Perm(), di.Mode().Perm())
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSymlinkRoundTrip(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "real.txt"), "target", 0o644)
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink("real.txt", link); err != nil {
		t.Skipf("symlinks unavailable on this host: %v", err)
	}

	var arch bytes.Buffer
	if err := Create(&arch, []string{root}, false, nil); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := Extract(bytes.NewReader(arch.Bytes()), dest, false, nil); err != nil {
		t.Fatal(err)
	}
	got := filepath.Join(dest, filepath.FromSlash(toArchiveName(link, false)))
	fi, err := os.Lstat(got)
	if err != nil {
		t.Fatalf("symlink not extracted: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected a symlink at %s", got)
	}
	tgt, err := os.Readlink(got)
	if err != nil || filepath.ToSlash(tgt) != "real.txt" {
		t.Errorf("link target = %q (%v), want real.txt", tgt, err)
	}
}

func TestListOutput(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "one.txt"), "1", 0o644)
	mustWrite(t, filepath.Join(root, "sub", "two.txt"), "22", 0o644)

	var arch bytes.Buffer
	if err := Create(&arch, []string{root}, false, nil); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := List(bytes.NewReader(arch.Bytes()), &out, false); err != nil {
		t.Fatal(err)
	}
	listing := out.String()
	for _, want := range []string{"one.txt", "sub/", "sub/two.txt"} {
		if !strings.Contains(listing, want) {
			t.Errorf("listing missing %q:\n%s", want, listing)
		}
	}

	// verbose long format should include mode chars and the size column
	var vout bytes.Buffer
	if err := List(bytes.NewReader(arch.Bytes()), &vout, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(vout.String(), "root/root") {
		t.Errorf("verbose listing missing owner column:\n%s", vout.String())
	}
}
