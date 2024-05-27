// Package cache keeps each repository's commits on disk so later runs only ask
// git for new ones. It stores raw commits, so changing the author, timezone or
// range options doesn't invalidate it.
package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitgraph/internal/model"
)

type commitJSON struct {
	SHA   string `json:"sha"`
	Name  string `json:"name"`
	Email string `json:"email"`
	When  string `json:"when"` // RFC3339
}

// RepoCache is what is cached for one repository.
type RepoCache struct {
	LastSHA   string       `json:"last_sha"`   // newest commit already cached
	SinceUnix int64        `json:"since_unix"` // earliest instant this cache covers
	Commits   []commitJSON `json:"commits"`
}

// Cache is the whole cache file. Repos is keyed by absolute repo path.
type Cache struct {
	Version int                  `json:"version"`
	Repos   map[string]RepoCache `json:"repos"`
}

const version = 1

// DefaultPath returns gitgraph/cache.json in the user cache directory.
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gitgraph", "cache.json"), nil
}

// Load reads the cache file. A missing, corrupt or outdated file gives an
// empty cache rather than an error; a bad cache should only make a run slower.
func Load(path string) *Cache {
	c := &Cache{Version: version, Repos: map[string]RepoCache{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var loaded Cache
	if err := json.Unmarshal(data, &loaded); err != nil || loaded.Version != version {
		return c
	}
	if loaded.Repos == nil {
		loaded.Repos = map[string]RepoCache{}
	}
	return &loaded
}

// Save writes the cache to path via a temp file and rename.
func (c *Cache) Save(path string) error {
	if c.Version == 0 {
		c.Version = version
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Get returns the cached commits for a repo and the newest cached SHA. ok is
// false when there is no entry or the cache doesn't reach back to since.
func (c *Cache) Get(repoPath string, since time.Time) (commits []model.Commit, lastSHA string, ok bool) {
	rc, found := c.Repos[repoPath]
	if !found || rc.LastSHA == "" {
		return nil, "", false
	}
	if !since.IsZero() && since.Unix() < rc.SinceUnix {
		return nil, "", false
	}
	return toModel(rc.Commits), rc.LastSHA, true
}

// Put replaces a repo's entry. newest is the most recent SHA and since is how
// far back commits covers.
func (c *Cache) Put(repoPath string, commits []model.Commit, newest string, since time.Time) {
	if c.Repos == nil {
		c.Repos = map[string]RepoCache{}
	}
	c.Repos[repoPath] = RepoCache{
		LastSHA:   newest,
		SinceUnix: since.Unix(),
		Commits:   fromModel(commits),
	}
}

// Merge returns newCommits followed by cached, without duplicate SHAs.
func Merge(newCommits, cached []model.Commit) []model.Commit {
	seen := make(map[string]struct{}, len(newCommits)+len(cached))
	out := make([]model.Commit, 0, len(newCommits)+len(cached))
	for _, c := range append(append([]model.Commit{}, newCommits...), cached...) {
		if c.SHA != "" {
			if _, dup := seen[c.SHA]; dup {
				continue
			}
			seen[c.SHA] = struct{}{}
		}
		out = append(out, c)
	}
	return out
}

func toModel(in []commitJSON) []model.Commit {
	out := make([]model.Commit, 0, len(in))
	for _, c := range in {
		when, err := time.Parse(time.RFC3339, c.When)
		if err != nil {
			continue
		}
		out = append(out, model.Commit{SHA: c.SHA, AuthorName: c.Name, AuthorEmail: c.Email, When: when})
	}
	return out
}

func fromModel(in []model.Commit) []commitJSON {
	out := make([]commitJSON, 0, len(in))
	for _, c := range in {
		out = append(out, commitJSON{SHA: c.SHA, Name: c.AuthorName, Email: c.AuthorEmail, When: c.When.Format(time.RFC3339)})
	}
	return out
}

// HashKey returns a short, stable hash of s.
func HashKey(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum[:8])
}
