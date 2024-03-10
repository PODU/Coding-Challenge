// Package imagefs unpacks image layers (gzipped tars) on top of each other into
// one directory. It applies whiteouts and refuses entries that would escape the
// target directory. Device nodes and file ownership are ignored, so it also
// works on Windows.
package imagefs

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// isUnsupportedLink reports whether creating a link failed because the OS
// doesn't allow it (e.g. symlinks on Windows without the right privilege).
// Those failures are ignored.
func isUnsupportedLink(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, errors.ErrUnsupported)
}

// Whiteouts: a file ".wh.<name>" in a layer deletes <name> from the layers
// below, and ".wh..wh..opq" empties its directory.
const (
	whiteoutPrefix = ".wh."
	whiteoutOpaque = ".wh..wh..opq"
)

// Stack extracts layers into destDir in order, so later layers win.
func Stack(layers []io.Reader, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create rootfs dir: %w", err)
	}
	for i, layer := range layers {
		if err := Extract(layer, destDir); err != nil {
			return fmt.Errorf("extract layer %d: %w", i, err)
		}
	}
	return nil
}

// Extract unpacks one layer into destDir.
func Extract(r io.Reader, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		if err := applyEntry(tr, hdr, destDir); err != nil {
			return err
		}
	}
}

func applyEntry(tr *tar.Reader, hdr *tar.Header, destDir string) error {
	target, err := secureJoin(destDir, hdr.Name)
	if err != nil {
		return err
	}

	base := filepath.Base(hdr.Name)

	if base == whiteoutOpaque {
		return clearDir(filepath.Dir(target))
	}
	if strings.HasPrefix(base, whiteoutPrefix) {
		victim := filepath.Join(filepath.Dir(target), strings.TrimPrefix(base, whiteoutPrefix))
		if err := os.RemoveAll(victim); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("apply whiteout %s: %w", hdr.Name, err)
		}
		return nil
	}

	switch hdr.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(target, os.FileMode(hdr.Mode)&os.ModePerm); err != nil {
			return fmt.Errorf("mkdir %s: %w", hdr.Name, err)
		}

	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("mkdir parent of %s: %w", hdr.Name, err)
		}
		// Remove whatever a lower layer put here; it might not even be a file.
		if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("replace %s: %w", hdr.Name, err)
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&os.ModePerm)
		if err != nil {
			return fmt.Errorf("create %s: %w", hdr.Name, err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return fmt.Errorf("write %s: %w", hdr.Name, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close %s: %w", hdr.Name, err)
		}

	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("mkdir parent of %s: %w", hdr.Name, err)
		}
		_ = os.RemoveAll(target)
		if err := os.Symlink(hdr.Linkname, target); err != nil && !isUnsupportedLink(err) {
			return fmt.Errorf("symlink %s: %w", hdr.Name, err)
		}

	case tar.TypeLink:
		linkTarget, err := secureJoin(destDir, hdr.Linkname)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("mkdir parent of %s: %w", hdr.Name, err)
		}
		_ = os.RemoveAll(target)
		if err := os.Link(linkTarget, target); err != nil && !isUnsupportedLink(err) {
			return fmt.Errorf("hardlink %s: %w", hdr.Name, err)
		}

	default:
		// Devices and FIFOs are skipped; ordinary images run fine without them.
		return nil
	}

	return nil
}

// secureJoin joins name onto destDir, rejecting names that would end up
// outside it.
func secureJoin(destDir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe absolute path in archive: %q", name)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal attempt in archive: %q", name)
	}
	return filepath.Join(destDir, clean), nil
}

// clearDir empties dir without removing it.
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dir for opaque whiteout %s: %w", dir, err)
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return fmt.Errorf("opaque whiteout %s: %w", dir, err)
		}
	}
	return nil
}
