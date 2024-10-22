package main

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"hash"
)

// FileInfo is what the pipeline keeps about each file. Dev and Ino are zero
// where the platform doesn't provide them.
type FileInfo struct {
	Path string
	Size int64
	Dev  uint64
	Ino  uint64
}

// DuplicateGroup is a set of files with identical content.
type DuplicateGroup struct {
	Size   int64    `json:"size"`
	Hash   string   `json:"hash"`
	Paths  []string `json:"paths"`
	Wasted int64    `json:"wasted_bytes"` // Size * (len(Paths)-1)

	members []FileInfo
}

// Summary holds the totals for a run.
type Summary struct {
	Groups      int   `json:"groups"`
	Files       int   `json:"files"`
	WastedBytes int64 `json:"wasted_bytes"`
}

// Report is the result of a scan, as written by --format json.
type Report struct {
	Groups  []DuplicateGroup `json:"groups"`
	Errors  []string         `json:"errors,omitempty"`
	Summary Summary          `json:"summary"`
}

// Stats counts how many files are left after each stage. Tests use it to check
// that the cheap stages weed out files before the expensive ones.
type Stats struct {
	Walked            int
	SizeCandidates    int
	PartialCandidates int
	FullCandidates    int
	Confirmed         int
}

// Options controls a scan.
type Options struct {
	MinSize        int64
	Algo           string // "md5" (default) or "sha256"
	FollowSymlinks bool
	PartialSize    int64 // bytes hashed in the partial stage; default 4096
	Workers        int   // default GOMAXPROCS
}

// ScanError is a problem with one path. Scanning carries on past it.
type ScanError struct {
	Path string
	Err  error
}

func (e ScanError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

func newHasher(algo string) (func() hash.Hash, error) {
	switch algo {
	case "", "md5":
		return md5.New, nil
	case "sha256":
		return sha256.New, nil
	default:
		return nil, fmt.Errorf("unknown hash algorithm %q (want md5 or sha256)", algo)
	}
}
