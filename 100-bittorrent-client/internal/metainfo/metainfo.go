// Package metainfo parses single-file .torrent files.
package metainfo

import (
	"crypto/sha1"
	"errors"
	"fmt"

	"bittorrent/internal/bencode"
)

// PieceHashLen is the size of one SHA-1 piece hash.
const PieceHashLen = 20

// Torrent is the metainfo needed to download a single file.
type Torrent struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int64
	Length      int64
	Name        string
}

// Parse decodes a .torrent file. Multi-file torrents are rejected.
func Parse(data []byte) (*Torrent, error) {
	root, raw, err := bencode.DecodeWithRaw(data)
	if err != nil {
		return nil, fmt.Errorf("metainfo: decode: %w", err)
	}
	top, ok := root.(map[string]interface{})
	if !ok {
		return nil, errors.New("metainfo: top-level value is not a dictionary")
	}

	announce, _ := top["announce"].(string)

	infoRaw, ok := raw["info"]
	if !ok {
		return nil, errors.New("metainfo: missing info dictionary")
	}
	info, ok := top["info"].(map[string]interface{})
	if !ok {
		return nil, errors.New("metainfo: info is not a dictionary")
	}

	if _, multi := info["files"]; multi {
		return nil, errors.New("metainfo: multi-file torrents are not supported (single-file only)")
	}

	name, ok := info["name"].(string)
	if !ok || name == "" {
		return nil, errors.New("metainfo: missing or invalid info.name")
	}
	pieceLength, ok := info["piece length"].(int64)
	if !ok || pieceLength <= 0 {
		return nil, errors.New("metainfo: missing or invalid info.piece length")
	}
	length, ok := info["length"].(int64)
	if !ok || length < 0 {
		return nil, errors.New("metainfo: missing or invalid info.length")
	}
	piecesStr, ok := info["pieces"].(string)
	if !ok {
		return nil, errors.New("metainfo: missing info.pieces")
	}
	pieces := []byte(piecesStr)
	if len(pieces)%PieceHashLen != 0 {
		return nil, fmt.Errorf("metainfo: pieces length %d is not a multiple of %d", len(pieces), PieceHashLen)
	}

	numPieces := len(pieces) / PieceHashLen
	hashes := make([][20]byte, numPieces)
	for i := 0; i < numPieces; i++ {
		copy(hashes[i][:], pieces[i*PieceHashLen:(i+1)*PieceHashLen])
	}

	expected := int64(0)
	if length > 0 {
		expected = (length + pieceLength - 1) / pieceLength
	}
	if int64(numPieces) != expected {
		return nil, fmt.Errorf("metainfo: piece count %d does not match length %d / piece length %d (expected %d)",
			numPieces, length, pieceLength, expected)
	}

	return &Torrent{
		Announce:    announce,
		InfoHash:    sha1.Sum(infoRaw),
		PieceHashes: hashes,
		PieceLength: pieceLength,
		Length:      length,
		Name:        name,
	}, nil
}

// NumPieces returns the number of pieces.
func (t *Torrent) NumPieces() int {
	return len(t.PieceHashes)
}

// PieceSize returns the size of piece index. The last piece may be short.
func (t *Torrent) PieceSize(index int) int64 {
	if index < 0 || index >= len(t.PieceHashes) {
		return 0
	}
	begin := int64(index) * t.PieceLength
	end := begin + t.PieceLength
	if end > t.Length {
		end = t.Length
	}
	return end - begin
}
