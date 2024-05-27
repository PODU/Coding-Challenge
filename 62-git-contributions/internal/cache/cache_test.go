package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitgraph/internal/model"
)

func tt(s string) time.Time {
	v, _ := time.Parse(time.RFC3339, s)
	return v
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	c := &Cache{Version: version, Repos: map[string]RepoCache{}}
	commits := []model.Commit{
		{SHA: "newest", AuthorName: "Ada", AuthorEmail: "a@x", When: tt("2026-03-02T10:00:00Z")},
		{SHA: "older", AuthorName: "Ada", AuthorEmail: "a@x", When: tt("2026-03-01T10:00:00Z")},
	}
	since := tt("2026-02-01T00:00:00Z")
	c.Put("/repo", commits, "newest", since)
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}

	got := Load(path)
	cached, last, ok := got.Get("/repo", since)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if last != "newest" {
		t.Errorf("lastSHA = %q, want newest", last)
	}
	if len(cached) != 2 || cached[0].SHA != "newest" {
		t.Errorf("cached commits wrong: %+v", cached)
	}
}

func TestGetWindowBeforeCoverageMisses(t *testing.T) {
	c := &Cache{Version: version, Repos: map[string]RepoCache{}}
	c.Put("/repo", []model.Commit{{SHA: "x", When: tt("2026-03-01T00:00:00Z")}}, "x", tt("2026-02-01T00:00:00Z"))

	// Requesting an earlier window than cached coverage => miss (full read needed).
	if _, _, ok := c.Get("/repo", tt("2026-01-01T00:00:00Z")); ok {
		t.Error("expected miss when window predates cached coverage")
	}
	// Same/later window => hit.
	if _, _, ok := c.Get("/repo", tt("2026-02-15T00:00:00Z")); !ok {
		t.Error("expected hit when window within cached coverage")
	}
}

func TestMergeDedup(t *testing.T) {
	cached := []model.Commit{
		{SHA: "b", When: tt("2026-03-01T00:00:00Z")},
		{SHA: "a", When: tt("2026-02-28T00:00:00Z")},
	}
	fresh := []model.Commit{
		{SHA: "c", When: tt("2026-03-03T00:00:00Z")},
		{SHA: "b", When: tt("2026-03-01T00:00:00Z")}, // overlaps cached
	}
	merged := Merge(fresh, cached)
	if len(merged) != 3 {
		t.Fatalf("merged len = %d, want 3 (deduped)", len(merged))
	}
	if merged[0].SHA != "c" {
		t.Errorf("fresh commits should come first, got %q", merged[0].SHA)
	}
}

func TestLoadCorruptIsColdStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Load(path)
	if c == nil || len(c.Repos) != 0 {
		t.Errorf("corrupt cache should load as empty, got %+v", c)
	}
}

func TestLoadMissingIsColdStart(t *testing.T) {
	c := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if c == nil || len(c.Repos) != 0 {
		t.Errorf("missing cache should load as empty")
	}
}

func TestGetUnknownRepoMisses(t *testing.T) {
	c := &Cache{Version: version, Repos: map[string]RepoCache{}}
	if _, _, ok := c.Get("/never-seen", time.Time{}); ok {
		t.Error("unknown repo should miss")
	}
}

func TestPutOnNilReposMap(t *testing.T) {
	c := &Cache{} // nil Repos
	c.Put("/r", []model.Commit{{SHA: "x", When: tt("2026-03-01T00:00:00Z")}}, "x", time.Time{})
	if _, _, ok := c.Get("/r", time.Time{}); !ok {
		t.Error("Put should initialise the map and store the entry")
	}
}

func TestToModelSkipsBadTime(t *testing.T) {
	c := &Cache{Version: version, Repos: map[string]RepoCache{
		"/r": {LastSHA: "x", Commits: []commitJSON{
			{SHA: "good", When: "2026-03-01T00:00:00Z"},
			{SHA: "bad", When: "not-a-time"},
		}},
	}}
	got, _, ok := c.Get("/r", time.Time{})
	if !ok {
		t.Fatal("expected hit")
	}
	if len(got) != 1 || got[0].SHA != "good" {
		t.Errorf("bad-time commit not skipped: %+v", got)
	}
}

func TestHashKeyStable(t *testing.T) {
	if HashKey("abc") != HashKey("abc") {
		t.Error("HashKey not stable")
	}
	if HashKey("abc") == HashKey("abd") {
		t.Error("HashKey collision on distinct input")
	}
}

func TestDefaultPath(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Skipf("no user cache dir: %v", err)
	}
	if !filepath.IsAbs(p) {
		t.Errorf("DefaultPath not absolute: %s", p)
	}
}
