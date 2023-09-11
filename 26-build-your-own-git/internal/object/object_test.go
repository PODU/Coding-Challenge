package object

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHashKnownBlobs(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"empty", "", EmptyBlobOID},
		{"hello", "hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
		{"what is up", "what is up, doc?", "bd9dbf5aae1a3862dd1526723246b20206e5fc37"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Hash(TypeBlob, []byte(c.payload))
			if got != c.want {
				t.Fatalf("Hash(blob, %q) = %s, want %s", c.payload, got, c.want)
			}
		})
	}
}

func TestEmptyTreeOID(t *testing.T) {
	payload, err := EncodeTree(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) != 0 {
		t.Fatalf("empty tree payload should be empty, got %d bytes", len(payload))
	}
	if got := Hash(TypeTree, payload); got != EmptyTreeOID {
		t.Fatalf("empty tree OID = %s, want %s", got, EmptyTreeOID)
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	gitDir := initGitDir(t)
	payload := []byte("round trip content\n")

	oid, err := Write(gitDir, TypeBlob, payload)
	if err != nil {
		t.Fatal(err)
	}
	if oid != Hash(TypeBlob, payload) {
		t.Fatalf("Write returned %s, Hash gives %s", oid, Hash(TypeBlob, payload))
	}

	typ, got, err := Read(gitDir, oid)
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeBlob {
		t.Fatalf("type = %s, want blob", typ)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: %q vs %q", got, payload)
	}
}

func TestLooseObjectBytesDecompress(t *testing.T) {
	gitDir := initGitDir(t)
	payload := []byte("hello\n")
	oid, err := Write(gitDir, TypeBlob, payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(gitDir, "objects", oid[:2], oid[2:]))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	full, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte("blob 6\x00"), payload...)
	if !bytes.Equal(full, want) {
		t.Fatalf("decompressed = %q, want %q", full, want)
	}
}

func TestTreeRoundTrip(t *testing.T) {
	entries := []TreeEntry{
		{Mode: ModeFile, Name: "zeta.txt", OID: "ce013625030ba8dba906f756967f9e9ca394464a"},
		{Mode: ModeTree, Name: "src", OID: EmptyTreeOID},
		{Mode: ModeFile, Name: "alpha.txt", OID: EmptyBlobOID},
	}
	payload, err := EncodeTree(entries)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeTree(payload)
	if err != nil {
		t.Fatal(err)
	}
	// Expect canonical ordering: alpha.txt, src, zeta.txt.
	wantNames := []string{"alpha.txt", "src", "zeta.txt"}
	if len(got) != len(wantNames) {
		t.Fatalf("got %d entries, want %d", len(got), len(wantNames))
	}
	for i, n := range wantNames {
		if got[i].Name != n {
			t.Fatalf("entry %d = %s, want %s", i, got[i].Name, n)
		}
	}
}

func TestTreeSortDirSuffix(t *testing.T) {
	// "foo" file vs "foo" dir ordering: a file named "foo.txt" should sort after
	// a tree named "foo" because the tree compares as "foo/".
	entries := []TreeEntry{
		{Mode: ModeFile, Name: "foo.txt", OID: EmptyBlobOID},
		{Mode: ModeTree, Name: "foo", OID: EmptyTreeOID},
	}
	payload, err := EncodeTree(entries)
	if err != nil {
		t.Fatal(err)
	}
	// Verified against canonical git: "foo.txt" sorts before the "foo" tree
	// because the tree compares as "foo/" and '.' (0x2E) < '/' (0x2F).
	got, _ := DecodeTree(payload)
	if got[0].Name != "foo.txt" || got[1].Name != "foo" {
		t.Fatalf("ordering wrong: %s, %s", got[0].Name, got[1].Name)
	}
}

func TestCommitRoundTrip(t *testing.T) {
	when := time.Unix(1700000000, 0).In(time.FixedZone("+0530", 5*3600+30*60))
	c := Commit{
		Tree:    EmptyTreeOID,
		Parents: []string{"ce013625030ba8dba906f756967f9e9ca394464a"},
		Author:  Signature{Name: "Ada Lovelace", Email: "ada@example.com", When: when},
		Committer: Signature{
			Name: "Ada Lovelace", Email: "ada@example.com", When: when,
		},
		Message: "Initial commit\n",
	}
	payload := EncodeCommit(c)
	got, err := DecodeCommit(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tree != c.Tree {
		t.Fatalf("tree = %s, want %s", got.Tree, c.Tree)
	}
	if len(got.Parents) != 1 || got.Parents[0] != c.Parents[0] {
		t.Fatalf("parents = %v", got.Parents)
	}
	if got.Author.Name != "Ada Lovelace" || got.Author.Email != "ada@example.com" {
		t.Fatalf("author = %+v", got.Author)
	}
	if !got.Author.When.Equal(when) {
		t.Fatalf("author time = %v, want %v", got.Author.When, when)
	}
	if got.Message != "Initial commit\n" {
		t.Fatalf("message = %q", got.Message)
	}

	// Re-encoding must be byte-identical (canonical form).
	if !bytes.Equal(EncodeCommit(got), payload) {
		t.Fatalf("re-encode not stable")
	}
}

func TestCollectReachable(t *testing.T) {
	gitDir := initGitDir(t)
	blobOID, _ := Write(gitDir, TypeBlob, []byte("hi\n"))
	treePayload, _ := EncodeTree([]TreeEntry{{Mode: ModeFile, Name: "a.txt", OID: blobOID}})
	treeOID, _ := Write(gitDir, TypeTree, treePayload)
	when := time.Unix(1700000000, 0).UTC()
	c := Commit{
		Tree:      treeOID,
		Author:    Signature{Name: "T", Email: "t@e", When: when},
		Committer: Signature{Name: "T", Email: "t@e", When: when},
		Message:   "c1\n",
	}
	commitOID, _ := Write(gitDir, TypeCommit, EncodeCommit(c))

	objs, err := CollectReachable(gitDir, []string{commitOID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 3 {
		t.Fatalf("reachable = %d objects, want 3 (commit, tree, blob)", len(objs))
	}

	// With the commit already present remotely, nothing should be collected.
	objs2, err := CollectReachable(gitDir, []string{commitOID}, map[string]bool{commitOID: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(objs2) != 0 {
		t.Fatalf("expected 0 objects when remote has tip, got %d", len(objs2))
	}
}

// initGitDir creates a minimal .git/objects layout in a temp dir.
func initGitDir(t *testing.T) string {
	t.Helper()
	gitDir := filepath.Join(t.TempDir(), ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	return gitDir
}
