package shortener

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store maps short keys to long URLs.
type Store interface {
	Save(key, longURL string) error
	Get(key string) (string, bool)
	// Delete reports whether the key existed.
	Delete(key string) bool
	FindByURL(longURL string) (string, bool)
}

// MemStore is an in-memory Store, safe for concurrent use. It keeps a reverse
// index so FindByURL is a map lookup.
type MemStore struct {
	mu       sync.RWMutex
	keyToURL map[string]string
	urlToKey map[string]string
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{
		keyToURL: make(map[string]string),
		urlToKey: make(map[string]string),
	}
}

func (s *MemStore) Save(key, longURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keyToURL[key] = longURL
	s.urlToKey[longURL] = key
	return nil
}

func (s *MemStore) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.keyToURL[key]
	return u, ok
}

func (s *MemStore) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.keyToURL[key]
	if !ok {
		return false
	}
	delete(s.keyToURL, key)
	delete(s.urlToKey, u)
	return true
}

func (s *MemStore) FindByURL(longURL string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.urlToKey[longURL]
	return k, ok
}

func (s *MemStore) snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.keyToURL))
	for k, v := range s.keyToURL {
		out[k] = v
	}
	return out
}

// FileStore is a MemStore that rewrites a JSON file after every change. It
// writes to a temp file and renames it, so a crash mid-write can't leave a
// half-written store behind.
type FileStore struct {
	*MemStore
	path    string
	writeMu sync.Mutex
}

// NewFileStore loads path if it exists; a missing file means an empty store.
func NewFileStore(path string) (*FileStore, error) {
	fs := &FileStore{MemStore: NewMemStore(), path: path}
	if err := fs.load(); err != nil {
		return nil, err
	}
	return fs, nil
}

func (fs *FileStore) load() error {
	data, err := os.ReadFile(fs.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	for k, u := range m {
		_ = fs.MemStore.Save(k, u)
	}
	return nil
}

func (fs *FileStore) persist() error {
	fs.writeMu.Lock()
	defer fs.writeMu.Unlock()

	data, err := json.MarshalIndent(fs.MemStore.snapshot(), "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(fs.path)
	tmp, err := os.CreateTemp(dir, ".urlstore-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, fs.path)
}

func (fs *FileStore) Save(key, longURL string) error {
	if err := fs.MemStore.Save(key, longURL); err != nil {
		return err
	}
	return fs.persist()
}

func (fs *FileStore) Delete(key string) bool {
	if !fs.MemStore.Delete(key) {
		return false
	}
	// Best effort: the delete has already happened in memory.
	_ = fs.persist()
	return true
}

var _ Store = (*MemStore)(nil)
var _ Store = (*FileStore)(nil)
