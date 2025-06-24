// Package walk finds the source files to count.
package walk

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"loc/internal/lang"
)

// Options controls which files Walk returns.
type Options struct {
	NoIgnore       bool // don't skip .git, node_modules, vendor and so on
	IncludeUnknown bool
	Excludes       []string // globs
	Langs          []string // if set, only these languages
}

// Job is a file to count and its language.
type Job struct {
	Path string
	Def  *lang.LanguageDef
}

var defaultIgnoreDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, ".codegraph": true,
}

// Walk returns the source files under roots. Symlinks are not followed, which
// also rules out loops.
func Walk(roots []string, opt Options) ([]Job, error) {
	var jobs []Job
	for _, root := range roots {
		info, err := os.Lstat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if j, ok := consider(root, opt); ok {
				jobs = append(jobs, j)
			}
			continue
		}
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != root && !opt.NoIgnore && defaultIgnoreDirs[d.Name()] {
					return fs.SkipDir
				}
				if excluded(path, opt.Excludes) {
					return fs.SkipDir
				}
				return nil
			}
			if d.Type()&fs.ModeSymlink != 0 {
				return nil
			}
			if excluded(path, opt.Excludes) {
				return nil
			}
			if j, ok := consider(path, opt); ok {
				jobs = append(jobs, j)
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return jobs, nil
}

// consider decides whether a single file should be counted.
func consider(path string, opt Options) (Job, bool) {
	def, ok := lang.Detect(path)
	if !ok {
		if !opt.IncludeUnknown {
			return Job{}, false
		}
		def = lang.UnknownDef()
	}
	if len(opt.Langs) > 0 && !matchLang(def.Name, opt.Langs) {
		return Job{}, false
	}
	if isBinary(path) {
		return Job{}, false
	}
	return Job{Path: path, Def: def}, true
}

func matchLang(name string, langs []string) bool {
	for _, l := range langs {
		if strings.EqualFold(name, l) {
			return true
		}
	}
	return false
}

// excluded matches each glob against both the base name and the full path.
func excluded(path string, globs []string) bool {
	base := filepath.Base(path)
	for _, g := range globs {
		if ok, _ := filepath.Match(g, base); ok {
			return true
		}
		if ok, _ := filepath.Match(g, path); ok {
			return true
		}
	}
	return false
}

// isBinary treats a file as binary if its first 8 KB contain a NUL byte.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	return bytes.IndexByte(buf[:n], 0) >= 0
}
