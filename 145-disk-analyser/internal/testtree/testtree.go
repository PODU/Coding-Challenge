// Package testtree builds the challenge's sample tree in Go, so the tests run
// the same on Windows as on Unix (where the challenge uses head, ln, truncate
// and touch).
package testtree

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"
)

const MB = 1 << 20

// Files maps each regular file to its size.
var Files = map[string]int64{
	"docs/report.pdf":                           5 * MB,
	"media/video.bin":                           40 * MB,
	"media/photo1.bin":                          8 * MB,
	"media/photo2.bin":                          2 * MB,
	"projects/webapp/node_modules/lib/big.js":   20 * MB,
	"projects/webapp/node_modules/lib/other.js": 1 * MB,
	"projects/api/target/api.bin":               3 * MB,
	".cache/app/cache.db":                       10 * MB,
	"old/archive.bin":                           15 * MB,
}

var Small = map[string]string{
	"docs/notes.txt":             "some notes\n",
	"projects/webapp/src/app.js": "console.log('hi')\n",
	"locked/hidden.txt":          "secret\n",
}

// Result says which optional parts the platform allowed.
type Result struct {
	Symlink bool // docs/media-link -> ../media
	Sparse  bool // sparse.img is really sparse
}

// Build creates the tree under root. The unreadable "locked" directory is
// left readable here; tests simulate the permission error separately.
func Build(root string) (Result, error) {
	var res Result
	for _, d := range []string{"docs", "media", "old", "locked", "projects/webapp/src",
		"projects/webapp/node_modules/lib", "projects/api/target", ".cache/app"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return res, err
		}
	}
	rng := rand.NewChaCha8([32]byte{1})
	for name, size := range Files {
		buf := make([]byte, size)
		rng.Read(buf) // random so filesystem compression cannot shrink it
		if err := os.WriteFile(filepath.Join(root, name), buf, 0o644); err != nil {
			return res, err
		}
	}
	for name, s := range Small {
		if err := os.WriteFile(filepath.Join(root, name), []byte(s), 0o644); err != nil {
			return res, err
		}
	}
	if err := os.Link(filepath.Join(root, "media/video.bin"), filepath.Join(root, "media/video-link.bin")); err != nil {
		return res, err
	}
	res.Symlink = os.Symlink(filepath.Join("..", "media"), filepath.Join(root, "docs/media-link")) == nil
	old := time.Date(2019, 1, 1, 0, 0, 0, 0, time.Local)
	if err := os.Chtimes(filepath.Join(root, "old/archive.bin"), old, old); err != nil {
		return res, err
	}
	sparse, err := makeSparse(filepath.Join(root, "sparse.img"), 1<<30)
	if err != nil {
		return res, err
	}
	res.Sparse = sparse
	return res, nil
}
