package download

import (
	"bytes"
	"context"
	"crypto/sha1"
	"testing"
	"time"

	"bittorrent/internal/metainfo"
	"bittorrent/internal/testutil"
	"bittorrent/internal/tracker"
)

func TestCheckIntegrity(t *testing.T) {
	buf := []byte("the quick brown fox jumps over the lazy dog")
	pw := &pieceWork{index: 0, hash: sha1.Sum(buf), length: len(buf)}

	if err := checkIntegrity(pw, buf); err != nil {
		t.Errorf("good piece failed verification: %v", err)
	}

	corrupt := make([]byte, len(buf))
	copy(corrupt, buf)
	corrupt[0] ^= 0xFF
	if err := checkIntegrity(pw, corrupt); err == nil {
		t.Error("corrupt piece passed verification")
	}
}

// memWriterAt is an in-memory io.WriterAt backed by a fixed-size buffer.
type memWriterAt struct {
	buf []byte
}

func (w *memWriterAt) WriteAt(p []byte, off int64) (int, error) {
	copy(w.buf[off:], p)
	return len(p), nil
}

func runFakeDownload(t *testing.T, fileLen, pieceLen int, opts testutil.FakePeerOptions, numPeers int) {
	t.Helper()

	fileData, torrentBytes := testutil.MakeTorrent("test.bin", "http://fake/announce", fileLen, pieceLen)
	tr, err := metainfo.Parse(torrentBytes)
	if err != nil {
		t.Fatalf("parse torrent: %v", err)
	}

	out := &memWriterAt{buf: make([]byte, fileLen)}

	// Fabricate peer addresses; the fake dialer ignores them.
	peers := make([]tracker.Peer, numPeers)
	for i := range peers {
		peers[i] = tracker.Peer{IP: []byte{127, 0, 0, 1}, Port: uint16(7000 + i)}
	}

	var peerID [20]byte
	copy(peerID[:], "-GO0001-testtesttest")

	mgr := &Manager{
		Torrent:  tr,
		PeerID:   peerID,
		Timeout:  10 * time.Second,
		MaxPeers: numPeers,
		Dial:     testutil.FakeDialer(fileData, pieceLen, tr.NumPieces(), opts),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := mgr.Run(ctx, peers, out); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if !bytes.Equal(out.buf, fileData) {
		t.Error("assembled file does not match source")
	}
}

func TestDownloadFullFile(t *testing.T) {
	// 4 pieces, last partial: 64000 = 3*16384 + 14848.
	runFakeDownload(t, 64000, 16384, testutil.FakePeerOptions{CorruptPieceOnce: -1}, 1)
}

func TestDownloadWithCorruptRetry(t *testing.T) {
	// Piece 1 is served corrupt once, forcing a hash mismatch + requeue.
	runFakeDownload(t, 64000, 16384, testutil.FakePeerOptions{CorruptPieceOnce: 1}, 1)
}

func TestDownloadMultiplePeers(t *testing.T) {
	runFakeDownload(t, 100000, 16384, testutil.FakePeerOptions{CorruptPieceOnce: -1}, 4)
}

func TestDownloadSinglePiece(t *testing.T) {
	// File smaller than one piece: a single short piece.
	runFakeDownload(t, 5000, 16384, testutil.FakePeerOptions{CorruptPieceOnce: -1}, 1)
}
