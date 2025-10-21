// Package peer implements the BitTorrent peer wire protocol.
package peer

import (
	"errors"
	"fmt"
	"io"
)

// Pstr is the protocol string sent in the handshake.
const Pstr = "BitTorrent protocol"

// Handshake is the first message exchanged on a peer connection.
type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

// NewHandshake returns a handshake using Pstr.
func NewHandshake(infoHash, peerID [20]byte) *Handshake {
	return &Handshake{Pstr: Pstr, InfoHash: infoHash, PeerID: peerID}
}

// Serialize returns the wire form:
//
//	<pstrlen><pstr><8 reserved bytes><20-byte info_hash><20-byte peer_id>
func (h *Handshake) Serialize() []byte {
	buf := make([]byte, 1+len(h.Pstr)+8+20+20)
	buf[0] = byte(len(h.Pstr))
	n := 1
	n += copy(buf[n:], h.Pstr)
	n += 8 // reserved bytes, all zero
	n += copy(buf[n:], h.InfoHash[:])
	copy(buf[n:], h.PeerID[:])
	return buf
}

// ReadHandshake reads and parses a handshake from r.
func ReadHandshake(r io.Reader) (*Handshake, error) {
	lenBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}
	pstrLen := int(lenBuf[0])
	if pstrLen == 0 {
		return nil, errors.New("peer: handshake pstrlen is zero")
	}
	body := make([]byte, pstrLen+8+20+20)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	h := &Handshake{Pstr: string(body[:pstrLen])}
	copy(h.InfoHash[:], body[pstrLen+8:pstrLen+8+20])
	copy(h.PeerID[:], body[pstrLen+8+20:])
	return h, nil
}

// DoHandshake exchanges handshakes over rw and checks that the peer is on the
// same torrent.
func DoHandshake(rw io.ReadWriter, infoHash, peerID [20]byte) (*Handshake, error) {
	out := NewHandshake(infoHash, peerID)
	if _, err := rw.Write(out.Serialize()); err != nil {
		return nil, fmt.Errorf("peer: write handshake: %w", err)
	}
	in, err := ReadHandshake(rw)
	if err != nil {
		return nil, fmt.Errorf("peer: read handshake: %w", err)
	}
	if in.InfoHash != infoHash {
		return nil, fmt.Errorf("peer: handshake info hash mismatch: got %x want %x", in.InfoHash, infoHash)
	}
	return in, nil
}
