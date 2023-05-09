package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// persistEntry is one key in the snapshot file. The snapshot is plain JSON,
// not Redis's RDB format.
type persistEntry struct {
	Type      string   `json:"type"` // "string" or "list"
	Str       string   `json:"str,omitempty"`
	List      []string `json:"list,omitempty"`
	ExpiresAt int64    `json:"expires_at,omitempty"`
}

// Save writes live keys to path. It writes a temp file and renames it so a
// crash halfway through leaves the old snapshot intact.
func (s *Store) Save(path string) error {
	s.mu.RLock()
	now := s.now()
	snap := make(map[string]persistEntry, len(s.data))
	for k, e := range s.data {
		if isExpired(e, now) {
			continue
		}
		switch v := e.value.(type) {
		case string:
			snap[k] = persistEntry{Type: "string", Str: v, ExpiresAt: e.expiresAt}
		case []string:
			snap[k] = persistEntry{Type: "list", List: v, ExpiresAt: e.expiresAt}
		}
	}
	s.mu.RUnlock()

	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".dump-*.tmp")
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
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// Load replaces the dataset with the snapshot at path. A missing file is not
// an error.
func (s *Store) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var snap map[string]persistEntry
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("corrupt snapshot %q: %w", path, err)
	}

	loaded := make(map[string]entry, len(snap))
	for k, pe := range snap {
		switch pe.Type {
		case "string":
			loaded[k] = entry{value: pe.Str, expiresAt: pe.ExpiresAt}
		case "list":
			loaded[k] = entry{value: pe.List, expiresAt: pe.ExpiresAt}
		}
	}

	s.mu.Lock()
	s.data = loaded
	s.mu.Unlock()
	return nil
}
