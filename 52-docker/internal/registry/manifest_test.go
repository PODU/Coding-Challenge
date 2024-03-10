package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "manifests", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestParseIndexAndSelect(t *testing.T) {
	idx, err := ParseIndex(readFixture(t, "index.json"))
	if err != nil {
		t.Fatalf("ParseIndex: %v", err)
	}
	if len(idx.Manifests) != 2 {
		t.Fatalf("got %d manifests, want 2", len(idx.Manifests))
	}
	if !IsIndexMediaType(idx.MediaType) {
		t.Errorf("media type %q not detected as index", idx.MediaType)
	}

	desc, err := SelectManifest(idx, "linux", "amd64")
	if err != nil {
		t.Fatalf("SelectManifest: %v", err)
	}
	wantDigest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	if desc.Digest != wantDigest {
		t.Errorf("selected digest = %q; want %q", desc.Digest, wantDigest)
	}

	if _, err := SelectManifest(idx, "linux", "riscv64"); err == nil {
		t.Errorf("SelectManifest for missing platform should error")
	}
}

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest(readFixture(t, "manifest.json"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if !IsManifestMediaType(m.MediaType) {
		t.Errorf("media type %q not detected as manifest", m.MediaType)
	}
	if len(m.Layers) != 2 {
		t.Errorf("got %d layers, want 2", len(m.Layers))
	}
	if m.Config.Digest == "" {
		t.Errorf("manifest config digest empty")
	}
}

func TestParseImageConfig(t *testing.T) {
	c, err := ParseImageConfig(readFixture(t, "config.json"))
	if err != nil {
		t.Fatalf("ParseImageConfig: %v", err)
	}
	if c.Architecture != "amd64" || c.OS != "linux" {
		t.Errorf("platform = %s/%s", c.OS, c.Architecture)
	}
	if c.Config.WorkingDir != "/app" {
		t.Errorf("workdir = %q", c.Config.WorkingDir)
	}
	if len(c.Config.Entrypoint) != 1 || c.Config.Entrypoint[0] != "/docker-entrypoint.sh" {
		t.Errorf("entrypoint = %v", c.Config.Entrypoint)
	}
	if len(c.Config.Env) != 1 {
		t.Errorf("env = %v", c.Config.Env)
	}
}

func TestLooksLikeIndex(t *testing.T) {
	if !LooksLikeIndex(readFixture(t, "index.json")) {
		t.Errorf("index.json should look like an index")
	}
	if LooksLikeIndex(readFixture(t, "manifest.json")) {
		t.Errorf("manifest.json should not look like an index")
	}
}
