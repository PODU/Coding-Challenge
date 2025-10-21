package metainfo

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"bittorrent/internal/bencode"
)

const (
	goldenInfoHash  = "a5ffe2f81e52485d7399033923c70cde4a3f0547"
	goldenName      = "sample.bin"
	goldenLength    = 64000
	goldenPieceLen  = 16384
	goldenNumPieces = 4
	goldenLastPiece = 14848
)

func loadFixture(t *testing.T) *Torrent {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "sample.torrent")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	tr, err := Parse(data)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return tr
}

func TestInfoHashGolden(t *testing.T) {
	tr := loadFixture(t)
	got := hex.EncodeToString(tr.InfoHash[:])
	if got != goldenInfoHash {
		t.Errorf("info hash = %s, want %s", got, goldenInfoHash)
	}
}

func TestMetainfoFields(t *testing.T) {
	tr := loadFixture(t)
	if tr.Name != goldenName {
		t.Errorf("name = %q, want %q", tr.Name, goldenName)
	}
	if tr.Length != goldenLength {
		t.Errorf("length = %d, want %d", tr.Length, goldenLength)
	}
	if tr.PieceLength != goldenPieceLen {
		t.Errorf("piece length = %d, want %d", tr.PieceLength, goldenPieceLen)
	}
	if tr.NumPieces() != goldenNumPieces {
		t.Errorf("num pieces = %d, want %d", tr.NumPieces(), goldenNumPieces)
	}
}

func TestLastPieceSizing(t *testing.T) {
	tr := loadFixture(t)
	last := tr.PieceSize(tr.NumPieces() - 1)
	if last != goldenLastPiece {
		t.Errorf("last piece size = %d, want %d", last, goldenLastPiece)
	}
	// All non-final pieces are full length.
	for i := 0; i < tr.NumPieces()-1; i++ {
		if tr.PieceSize(i) != goldenPieceLen {
			t.Errorf("piece %d size = %d, want %d", i, tr.PieceSize(i), goldenPieceLen)
		}
	}
	// Sum of all piece sizes equals total length.
	var total int64
	for i := 0; i < tr.NumPieces(); i++ {
		total += tr.PieceSize(i)
	}
	if total != tr.Length {
		t.Errorf("sum of piece sizes = %d, want %d", total, tr.Length)
	}
}

func TestRejectMultiFile(t *testing.T) {
	info := map[string]interface{}{
		"name":         "dir",
		"piece length": int64(16384),
		"pieces":       string(make([]byte, 20)),
		"files": []interface{}{
			map[string]interface{}{"length": int64(1), "path": []interface{}{"a"}},
		},
	}
	data, _ := bencode.Encode(map[string]interface{}{"announce": "x", "info": info})
	if _, err := Parse(data); err == nil {
		t.Error("expected error for multi-file torrent")
	}
}

func TestRejectBadPiecesLength(t *testing.T) {
	info := map[string]interface{}{
		"name":         "x",
		"length":       int64(10),
		"piece length": int64(16384),
		"pieces":       string(make([]byte, 19)), // not a multiple of 20
	}
	data, _ := bencode.Encode(map[string]interface{}{"announce": "x", "info": info})
	if _, err := Parse(data); err == nil {
		t.Error("expected error for pieces length not multiple of 20")
	}
}

func TestInfoHashMatchesCanonicalReencode(t *testing.T) {
	// The info hash must be stable under canonical re-encoding of the parsed
	// fixture (proves the raw-span capture matches a canonical encode).
	path := filepath.Join("..", "..", "testdata", "sample.torrent")
	data, _ := os.ReadFile(path)
	root, _ := bencode.Decode(data)
	top := root.(map[string]interface{})
	reInfo, _ := bencode.Encode(top["info"])
	reTop := map[string]interface{}{"announce": top["announce"], "info": top["info"]}
	reData, _ := bencode.Encode(reTop)

	tr1, _ := Parse(data)
	tr2, _ := Parse(reData)
	if tr1.InfoHash != tr2.InfoHash {
		t.Errorf("info hash changed under re-encode: %x vs %x", tr1.InfoHash, tr2.InfoHash)
	}
	_ = reInfo
}
