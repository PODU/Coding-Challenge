package cmd

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestInterop_StdlibReadsMytar uses Go's archive/tar as an oracle: it must read
// a mytar-produced archive without error and report matching member metadata,
// including a member whose name requires a GNU long-name record.
func TestInterop_StdlibReadsMytar(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "alpha", 0o644)
	mustWrite(t, filepath.Join(root, "sub", "b.bin"), strings.Repeat("Q", 1000), 0o644)
	longName := strings.Repeat("L", 130) + ".txt"
	mustWrite(t, filepath.Join(root, longName), "long", 0o644)

	var arch bytes.Buffer
	if err := Create(&arch, []string{root}, false, nil); err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(bytes.NewReader(arch.Bytes()))
	got := map[string]int64{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("archive/tar failed to read mytar archive: %v", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("archive/tar body read: %v", err)
		}
		if int64(len(data)) != h.Size {
			t.Errorf("%s: read %d bytes, header says %d", h.Name, len(data), h.Size)
		}
		got[filepath.ToSlash(h.Name)] = h.Size
	}

	// Verify our three known members surfaced with correct sizes.
	wantSuffixes := map[string]int64{"a.txt": 5, "b.bin": 1000, longName: 4}
	for suffix, size := range wantSuffixes {
		found := false
		for name, sz := range got {
			if strings.HasSuffix(name, suffix) {
				found = true
				if sz != size {
					t.Errorf("%s size = %d, want %d", suffix, sz, size)
				}
			}
		}
		if !found {
			t.Errorf("stdlib did not surface member ending %q; got %v", suffix, got)
		}
	}
}

// TestInterop_MytarReadsStdlib builds a USTAR archive with archive/tar (a
// trusted writer) and verifies mytar lists and extracts it correctly.
func TestInterop_MytarReadsStdlib(t *testing.T) {
	var arch bytes.Buffer
	tw := tar.NewWriter(&arch)

	entries := []struct {
		name string
		body string
		dir  bool
	}{
		{"dir/", "", true},
		{"dir/file1.txt", "first", false},
		{"file2.txt", "second body", false},
	}
	mtime := time.Unix(1600000000, 0)
	for _, e := range entries {
		h := &tar.Header{
			Name:    e.name,
			Mode:    0o644,
			ModTime: mtime,
			Format:  tar.FormatUSTAR,
			Uname:   "root",
			Gname:   "root",
		}
		if e.dir {
			h.Typeflag = tar.TypeDir
			h.Mode = 0o755
		} else {
			h.Typeflag = tar.TypeReg
			h.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if e.body != "" {
			if _, err := io.WriteString(tw, e.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	// List
	var out bytes.Buffer
	if err := List(bytes.NewReader(arch.Bytes()), &out, false); err != nil {
		t.Fatalf("mytar List of stdlib archive: %v", err)
	}
	names := strings.Fields(strings.ReplaceAll(out.String(), "\n", " "))
	sort.Strings(names)
	want := []string{"dir/", "dir/file1.txt", "file2.txt"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("list = %v, want %v", names, want)
	}

	// Extract
	dest := t.TempDir()
	if err := Extract(bytes.NewReader(arch.Bytes()), dest, false, nil); err != nil {
		t.Fatalf("mytar Extract of stdlib archive: %v", err)
	}
	for _, e := range entries {
		p := filepath.Join(dest, filepath.FromSlash(e.name))
		if e.dir {
			fi, err := os.Stat(p)
			if err != nil || !fi.IsDir() {
				t.Errorf("dir %s not extracted: %v", e.name, err)
			}
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("file %s not extracted: %v", e.name, err)
			continue
		}
		if string(data) != e.body {
			t.Errorf("%s content = %q, want %q", e.name, data, e.body)
		}
	}
}
