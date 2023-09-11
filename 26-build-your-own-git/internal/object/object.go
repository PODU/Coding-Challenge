// Package object implements Git's content-addressed object database: loose
// object read/write (zlib-framed), SHA-1 hashing, and the blob/tree/commit
// payload codecs. OIDs are the SHA-1 of the uncompressed "<type> <size>\0<payload>".
package object

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Type is a Git object type.
type Type string

const (
	TypeBlob   Type = "blob"
	TypeTree   Type = "tree"
	TypeCommit Type = "commit"
	TypeTag    Type = "tag"
)

// Well-known OIDs.
const (
	EmptyBlobOID = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"
	EmptyTreeOID = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
)

// frame returns the canonical uncompressed object bytes: "<type> <size>\0<payload>".
func frame(t Type, payload []byte) []byte {
	header := fmt.Sprintf("%s %d\x00", t, len(payload))
	out := make([]byte, 0, len(header)+len(payload))
	out = append(out, header...)
	out = append(out, payload...)
	return out
}

// Hash returns the hex SHA-1 OID for an object of the given type and payload,
// without writing anything to disk.
func Hash(t Type, payload []byte) string {
	sum := sha1.Sum(frame(t, payload))
	return hex.EncodeToString(sum[:])
}

// looseObjectPath returns the on-disk path for an OID within gitDir.
func looseObjectPath(gitDir, oid string) string {
	return filepath.Join(gitDir, "objects", oid[:2], oid[2:])
}

// Exists reports whether the loose object is already stored.
func Exists(gitDir, oid string) bool {
	_, err := os.Stat(looseObjectPath(gitDir, oid))
	return err == nil
}

// Write stores an object as a loose, zlib-compressed file and returns its OID.
// Writing an object that already exists is a no-op (content-addressed).
func Write(gitDir string, t Type, payload []byte) (string, error) {
	full := frame(t, payload)
	sum := sha1.Sum(full)
	oid := hex.EncodeToString(sum[:])

	path := looseObjectPath(gitDir, oid)
	if _, err := os.Stat(path); err == nil {
		return oid, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(full); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}

	// Write to a temp file and rename, so readers never see a partial object.
	tmp, err := os.CreateTemp(filepath.Dir(path), "tmp_obj_*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, path); err != nil {
		// On Windows a concurrent writer may have created it first.
		os.Remove(tmpName)
		if _, statErr := os.Stat(path); statErr == nil {
			return oid, nil
		}
		return "", err
	}
	return oid, nil
}

// Read loads a loose object, returning its type and raw payload.
func Read(gitDir, oid string) (Type, []byte, error) {
	if len(oid) != 40 {
		return "", nil, fmt.Errorf("invalid object id %q", oid)
	}
	f, err := os.Open(looseObjectPath(gitDir, oid))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("object %s not found", oid)
		}
		return "", nil, err
	}
	defer f.Close()

	zr, err := zlib.NewReader(f)
	if err != nil {
		return "", nil, fmt.Errorf("decompress %s: %w", oid, err)
	}
	defer zr.Close()
	full, err := io.ReadAll(zr)
	if err != nil {
		return "", nil, err
	}

	nul := bytes.IndexByte(full, 0)
	if nul < 0 {
		return "", nil, fmt.Errorf("object %s: missing header terminator", oid)
	}
	header := string(full[:nul])
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("object %s: bad header %q", oid, header)
	}
	size, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", nil, fmt.Errorf("object %s: bad size: %w", oid, err)
	}
	payload := full[nul+1:]
	if len(payload) != size {
		return "", nil, fmt.Errorf("object %s: size mismatch (header %d, got %d)", oid, size, len(payload))
	}
	return Type(parts[0]), payload, nil
}

// Common file modes stored in trees.
const (
	ModeFile    = "100644"
	ModeExec    = "100755"
	ModeSymlink = "120000"
	ModeTree    = "40000"
)

// TreeEntry is one row of a tree object.
type TreeEntry struct {
	Mode string
	Name string
	OID  string // hex
}

// treeSortKey returns the byte sequence Git compares for ordering: tree entries
// sort as if their name had a trailing '/'.
func treeSortKey(e TreeEntry) string {
	if e.Mode == ModeTree {
		return e.Name + "/"
	}
	return e.Name
}

// EncodeTree serializes entries in Git's canonical sorted order.
func EncodeTree(entries []TreeEntry) ([]byte, error) {
	sorted := make([]TreeEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		return treeSortKey(sorted[i]) < treeSortKey(sorted[j])
	})

	var buf bytes.Buffer
	for _, e := range sorted {
		raw, err := hex.DecodeString(e.OID)
		if err != nil || len(raw) != 20 {
			return nil, fmt.Errorf("tree entry %q: bad oid %q", e.Name, e.OID)
		}
		buf.WriteString(e.Mode)
		buf.WriteByte(' ')
		buf.WriteString(e.Name)
		buf.WriteByte(0)
		buf.Write(raw)
	}
	return buf.Bytes(), nil
}

// DecodeTree parses a tree payload into entries.
func DecodeTree(payload []byte) ([]TreeEntry, error) {
	var entries []TreeEntry
	i := 0
	for i < len(payload) {
		sp := bytes.IndexByte(payload[i:], ' ')
		if sp < 0 {
			return nil, fmt.Errorf("tree: missing mode separator")
		}
		mode := string(payload[i : i+sp])
		i += sp + 1

		nul := bytes.IndexByte(payload[i:], 0)
		if nul < 0 {
			return nil, fmt.Errorf("tree: missing name terminator")
		}
		name := string(payload[i : i+nul])
		i += nul + 1

		if i+20 > len(payload) {
			return nil, fmt.Errorf("tree: truncated oid")
		}
		oid := hex.EncodeToString(payload[i : i+20])
		i += 20

		entries = append(entries, TreeEntry{Mode: mode, Name: name, OID: oid})
	}
	return entries, nil
}

// Signature is an author/committer identity with a timestamp.
type Signature struct {
	Name  string
	Email string
	When  time.Time
}

func (s Signature) encode() string {
	return fmt.Sprintf("%s <%s> %d %s", s.Name, s.Email, s.When.Unix(), s.When.Format("-0700"))
}

// Commit is a parsed commit object.
type Commit struct {
	Tree      string
	Parents   []string
	Author    Signature
	Committer Signature
	Message   string
}

// EncodeCommit serializes a commit object payload.
func EncodeCommit(c Commit) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "tree %s\n", c.Tree)
	for _, p := range c.Parents {
		fmt.Fprintf(&buf, "parent %s\n", p)
	}
	fmt.Fprintf(&buf, "author %s\n", c.Author.encode())
	fmt.Fprintf(&buf, "committer %s\n", c.Committer.encode())
	buf.WriteByte('\n')
	msg := c.Message
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	buf.WriteString(msg)
	return buf.Bytes()
}

// DecodeCommit parses a commit object payload.
func DecodeCommit(payload []byte) (Commit, error) {
	var c Commit
	text := string(payload)
	sep := strings.Index(text, "\n\n")
	if sep < 0 {
		return c, fmt.Errorf("commit: missing header/body separator")
	}
	headers := text[:sep]
	c.Message = text[sep+2:]

	for _, line := range strings.Split(headers, "\n") {
		kv := strings.SplitN(line, " ", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "tree":
			c.Tree = val
		case "parent":
			c.Parents = append(c.Parents, val)
		case "author":
			sig, err := parseSignature(val)
			if err != nil {
				return c, err
			}
			c.Author = sig
		case "committer":
			sig, err := parseSignature(val)
			if err != nil {
				return c, err
			}
			c.Committer = sig
		}
	}
	return c, nil
}

// parseSignature parses "Name <email> <unixtime> <tz>".
func parseSignature(s string) (Signature, error) {
	var sig Signature
	lt := strings.Index(s, "<")
	gt := strings.Index(s, ">")
	if lt < 0 || gt < 0 || gt < lt {
		return sig, fmt.Errorf("signature: bad format %q", s)
	}
	sig.Name = strings.TrimSpace(s[:lt])
	sig.Email = s[lt+1 : gt]
	rest := strings.Fields(strings.TrimSpace(s[gt+1:]))
	if len(rest) >= 2 {
		secs, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			return sig, fmt.Errorf("signature: bad time %q", rest[0])
		}
		loc := parseTZ(rest[1])
		sig.When = time.Unix(secs, 0).In(loc)
	} else {
		sig.When = time.Unix(0, 0).UTC()
	}
	return sig, nil
}

func parseTZ(tz string) *time.Location {
	if len(tz) != 5 || (tz[0] != '+' && tz[0] != '-') {
		return time.UTC
	}
	h, err1 := strconv.Atoi(tz[1:3])
	m, err2 := strconv.Atoi(tz[3:5])
	if err1 != nil || err2 != nil {
		return time.UTC
	}
	off := (h*60 + m) * 60
	if tz[0] == '-' {
		off = -off
	}
	return time.FixedZone(tz, off)
}
