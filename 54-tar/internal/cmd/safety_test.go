package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mytar/internal/archive"
	"mytar/internal/ustar"
)

// craftArchive builds a minimal archive with one regular-file member of the
// given (possibly malicious) name and body.
func craftArchive(t *testing.T, name, body string, typeflag byte, linkname string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := archive.NewWriter(&buf)
	h := &ustar.Header{
		Name:     name,
		Mode:     0o644,
		Size:     int64(len(body)),
		ModTime:  time.Unix(0, 0),
		Typeflag: typeflag,
		Linkname: linkname,
	}
	if !ustar.FitsUSTARName(name) {
		if err := w.WriteGNULongName(name); err != nil {
			t.Fatal(err)
		}
		if len(name) > 100 {
			h.Name = name[:100]
		}
	}
	if err := w.WriteHeader(h); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractRejectsDotDot(t *testing.T) {
	arch := craftArchive(t, "../evil.txt", "pwned", ustar.TypeReg, "")
	dest := t.TempDir()
	err := Extract(bytes.NewReader(arch), dest, false, nil)
	if err == nil {
		t.Fatal("expected extraction to refuse ../ path")
	}
	if !strings.Contains(err.Error(), "escapes destination") {
		t.Errorf("unexpected error: %v", err)
	}
	// Ensure nothing was written outside dest.
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(dest), "evil.txt")); statErr == nil {
		t.Error("file escaped the destination directory")
	}
}

func TestExtractRejectsAbsolutePath(t *testing.T) {
	name := "/etc/evil"
	if filepath.Separator == '\\' {
		name = "/Windows/evil"
	}
	arch := craftArchive(t, name, "x", ustar.TypeReg, "")
	dest := t.TempDir()
	if err := Extract(bytes.NewReader(arch), dest, false, nil); err == nil {
		t.Fatal("expected extraction to refuse absolute path")
	}
}

func TestExtractRejectsEscapingSymlink(t *testing.T) {
	arch := craftArchive(t, "link", "", ustar.TypeSymlink, "../../outside")
	dest := t.TempDir()
	if err := Extract(bytes.NewReader(arch), dest, false, nil); err == nil {
		t.Fatal("expected extraction to refuse escaping symlink")
	}
}

func TestSanitizePathUnit(t *testing.T) {
	dest := filepath.Clean(t.TempDir())
	good := []string{"a/b/c.txt", "x", "deep/nested/file"}
	for _, g := range good {
		if _, err := sanitizePath(dest, g); err != nil {
			t.Errorf("sanitizePath rejected safe name %q: %v", g, err)
		}
	}
	bad := []string{"../x", "a/../../x", "/abs/x"}
	for _, b := range bad {
		if _, err := sanitizePath(dest, b); err == nil {
			t.Errorf("sanitizePath accepted unsafe name %q", b)
		}
	}
}
