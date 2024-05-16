package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"dropbox/internal/atomic"
	"dropbox/internal/wire"
)

// Store errors.
var (
	ErrNotFound        = errors.New("not found")
	ErrVersionConflict = errors.New("version conflict")
	ErrHashMismatch    = errors.New("hash mismatch")
	ErrBadPath         = errors.New("bad path")
)

// Store keeps each user's files under <root>/<user>/files, with versions in
// <root>/<user>/index.json. Writes are atomic and version-checked.
type Store struct {
	root  string
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewStore(root string) *Store {
	return &Store{root: root, locks: map[string]*sync.Mutex{}}
}

func (s *Store) userLock(user string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[user]
	if !ok {
		l = &sync.Mutex{}
		s.locks[user] = l
	}
	return l
}

func (s *Store) userDir(user string) string   { return filepath.Join(s.root, safeUser(user)) }
func (s *Store) indexPath(user string) string { return filepath.Join(s.userDir(user), "index.json") }

func (s *Store) filePath(user, rel string) (string, error) {
	clean, err := cleanRel(rel)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.userDir(user), "files", filepath.FromSlash(clean)), nil
}

type storedIndex map[string]wire.FileInfo

func (s *Store) loadIndex(user string) (storedIndex, error) {
	b, err := os.ReadFile(s.indexPath(user))
	if err != nil {
		if os.IsNotExist(err) {
			return storedIndex{}, nil
		}
		return nil, err
	}
	idx := storedIndex{}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &idx); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

func (s *Store) saveIndex(user string, idx storedIndex) error {
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return atomic.WriteBytes(s.indexPath(user), 0o644, b)
}

// List returns the user's files sorted by path.
func (s *Store) List(user string) ([]wire.FileInfo, error) {
	l := s.userLock(user)
	l.Lock()
	defer l.Unlock()
	idx, err := s.loadIndex(user)
	if err != nil {
		return nil, err
	}
	out := make([]wire.FileInfo, 0, len(idx))
	for p, fi := range idx {
		fi.Path = p
		out = append(out, fi)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Get opens the file for reading. The caller must Close the returned file.
func (s *Store) Get(user, rel string) (*os.File, wire.FileInfo, error) {
	clean, err := cleanRel(rel)
	if err != nil {
		return nil, wire.FileInfo{}, err
	}
	l := s.userLock(user)
	l.Lock()
	idx, err := s.loadIndex(user)
	if err != nil {
		l.Unlock()
		return nil, wire.FileInfo{}, err
	}
	fi, ok := idx[clean]
	l.Unlock()
	if !ok {
		return nil, wire.FileInfo{}, ErrNotFound
	}
	fi.Path = clean
	fp, _ := s.filePath(user, clean)
	f, err := os.Open(fp)
	if err != nil {
		return nil, wire.FileInfo{}, err
	}
	return f, fi, nil
}

// Put stores r at rel. The upload goes to a temp file first; if declaredHash
// is set and doesn't match, nothing changes and ErrHashMismatch is returned.
// An ifMatch >= 0 must equal the current version (else ErrVersionConflict);
// a negative ifMatch overwrites unconditionally.
func (s *Store) Put(user, rel string, ifMatch int64, declaredHash string, r io.Reader) (wire.FileInfo, error) {
	clean, err := cleanRel(rel)
	if err != nil {
		return wire.FileInfo{}, err
	}

	tmpDir := filepath.Join(s.userDir(user), "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return wire.FileInfo{}, err
	}
	tmp, err := os.CreateTemp(tmpDir, "up-*")
	if err != nil {
		return wire.FileInfo{}, err
	}
	tmpName := tmp.Name()
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return wire.FileInfo{}, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return wire.FileInfo{}, err
	}
	tmp.Close()
	got := hex.EncodeToString(h.Sum(nil))
	if declaredHash != "" && declaredHash != got {
		os.Remove(tmpName)
		return wire.FileInfo{}, ErrHashMismatch
	}

	l := s.userLock(user)
	l.Lock()
	defer l.Unlock()
	idx, err := s.loadIndex(user)
	if err != nil {
		os.Remove(tmpName)
		return wire.FileInfo{}, err
	}
	cur, exists := idx[clean]
	var curVersion int64
	if exists {
		curVersion = cur.Version
	}
	if ifMatch >= 0 && ifMatch != curVersion {
		os.Remove(tmpName)
		return wire.FileInfo{}, ErrVersionConflict
	}
	dst, _ := s.filePath(user, clean)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		os.Remove(tmpName)
		return wire.FileInfo{}, err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return wire.FileInfo{}, err
	}
	fi := wire.FileInfo{Path: clean, Hash: got, Size: size, Version: curVersion + 1}
	idx[clean] = fi
	if err := s.saveIndex(user, idx); err != nil {
		return wire.FileInfo{}, err
	}
	return fi, nil
}

// Delete removes rel, checking ifMatch like Put. Deleting a missing path is
// not an error.
func (s *Store) Delete(user, rel string, ifMatch int64) error {
	clean, err := cleanRel(rel)
	if err != nil {
		return err
	}
	l := s.userLock(user)
	l.Lock()
	defer l.Unlock()
	idx, err := s.loadIndex(user)
	if err != nil {
		return err
	}
	cur, exists := idx[clean]
	if !exists {
		return nil
	}
	if ifMatch >= 0 && ifMatch != cur.Version {
		return ErrVersionConflict
	}
	dst, _ := s.filePath(user, clean)
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(idx, clean)
	return s.saveIndex(user, idx)
}

// cleanRel normalises a client-supplied path and rejects absolute paths, ""
// and ".", and anything containing "..".
func cleanRel(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", ErrBadPath
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	if strings.HasPrefix(rel, "/") {
		return "", ErrBadPath
	}
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", ErrBadPath
		}
		out = append(out, p)
	}
	clean := path.Clean(strings.Join(out, "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrBadPath
	}
	return clean, nil
}

// safeUser makes a user name safe to use as a directory name.
func safeUser(u string) string {
	u = strings.ReplaceAll(u, "/", "_")
	u = strings.ReplaceAll(u, "\\", "_")
	u = strings.ReplaceAll(u, "..", "_")
	if u == "" {
		return "_"
	}
	return u
}
