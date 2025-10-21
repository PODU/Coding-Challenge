// Package testutil builds in-memory torrents and a fake peer served over
// net.Pipe, so the download loop can be tested without a network.
package testutil

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"bittorrent/internal/bencode"
	"bittorrent/internal/peer"
)

// MakeTorrent returns fileLen bytes of repeatable pseudo-random data and a
// .torrent describing it.
func MakeTorrent(name, announce string, fileLen, pieceLen int) (fileData, torrentBytes []byte) {
	fileData = make([]byte, fileLen)
	// xorshift with a fixed seed
	var x uint64 = 0x9e3779b97f4a7c15
	for i := range fileData {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		fileData[i] = byte(x)
	}

	var pieces []byte
	for off := 0; off < fileLen; off += pieceLen {
		end := off + pieceLen
		if end > fileLen {
			end = fileLen
		}
		h := sha1.Sum(fileData[off:end])
		pieces = append(pieces, h[:]...)
	}

	info := map[string]interface{}{
		"name":         name,
		"length":       int64(fileLen),
		"piece length": int64(pieceLen),
		"pieces":       string(pieces),
	}
	top := map[string]interface{}{
		"announce": announce,
		"info":     info,
	}
	torrentBytes, err := bencode.Encode(top)
	if err != nil {
		panic(fmt.Sprintf("testutil: encode torrent: %v", err))
	}
	return fileData, torrentBytes
}

type FakePeerOptions struct {
	// CorruptPieceOnce, if >= 0, makes the peer send bad data for that piece
	// the first time it is requested and good data after that.
	CorruptPieceOnce int
}

// FakeDialer returns a dialer whose peers have every piece of fileData. It
// returns a plain func type rather than download.Dialer so that this package
// doesn't import download, which would create an import cycle in its tests.
func FakeDialer(fileData []byte, pieceLen, numPieces int, opts FakePeerOptions) func(addr string, infoHash, peerID [20]byte, timeout time.Duration) (*peer.Client, error) {
	var mu sync.Mutex
	corrupted := map[int]bool{}

	return func(addr string, infoHash, peerID [20]byte, timeout time.Duration) (*peer.Client, error) {
		clientConn, serverConn := net.Pipe()
		go runFakePeer(serverConn, infoHash, fileData, pieceLen, numPieces, opts, &mu, corrupted)
		return peer.NewClientConn(clientConn, infoHash, peerID)
	}
}

func runFakePeer(conn net.Conn, infoHash [20]byte, fileData []byte, pieceLen, numPieces int, opts FakePeerOptions, mu *sync.Mutex, corrupted map[int]bool) {
	defer conn.Close()

	hs, err := peer.ReadHandshake(conn)
	if err != nil || hs.InfoHash != infoHash {
		return
	}
	var fakeID [20]byte
	copy(fakeID[:], "-FAKE0-peerpeerpeer1")
	if _, err := conn.Write(peer.NewHandshake(infoHash, fakeID).Serialize()); err != nil {
		return
	}

	// Send a full bitfield advertising every piece.
	bf := make([]byte, (numPieces+7)/8)
	for i := 0; i < numPieces; i++ {
		peer.Bitfield(bf).SetPiece(i)
	}
	if _, err := conn.Write((&peer.Message{ID: peer.MsgBitfield, Payload: bf}).Serialize()); err != nil {
		return
	}

	// Decouple reads and writes so pipelined requests do not deadlock on the
	// synchronous pipe.
	reqCh := make(chan *peer.Message, 256)
	go func() {
		defer close(reqCh)
		for {
			msg, err := peer.ReadMessage(conn)
			if err != nil {
				return
			}
			if msg != nil && msg.ID == peer.MsgRequest {
				reqCh <- msg
			}
			// Control messages (interested/unchoke/have/etc.) are ignored.
		}
	}()

	// Unchoke the client so it begins requesting.
	if _, err := conn.Write((&peer.Message{ID: peer.MsgUnchoke}).Serialize()); err != nil {
		return
	}

	for req := range reqCh {
		index := int(binary.BigEndian.Uint32(req.Payload[0:4]))
		begin := int(binary.BigEndian.Uint32(req.Payload[4:8]))
		length := int(binary.BigEndian.Uint32(req.Payload[8:12]))

		start := index*pieceLen + begin
		if start < 0 || start+length > len(fileData) {
			return
		}
		block := make([]byte, length)
		copy(block, fileData[start:start+length])

		if opts.CorruptPieceOnce >= 0 && index == opts.CorruptPieceOnce {
			mu.Lock()
			already := corrupted[index]
			corrupted[index] = true
			mu.Unlock()
			if !already && length > 0 {
				block[0] ^= 0xFF // flip a byte to force a SHA-1 mismatch
			}
		}

		payload := make([]byte, 8+length)
		binary.BigEndian.PutUint32(payload[0:4], uint32(index))
		binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
		copy(payload[8:], block)
		if _, err := conn.Write((&peer.Message{ID: peer.MsgPiece, Payload: payload}).Serialize()); err != nil {
			return
		}
	}
}
