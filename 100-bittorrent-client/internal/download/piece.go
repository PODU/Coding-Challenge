package download

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"time"

	"bittorrent/internal/peer"
)

// BlockSize is the size of each block request. 16 KiB is what peers expect.
const BlockSize = 16384

// MaxBacklog is how many block requests we keep in flight per peer.
const MaxBacklog = 5

type pieceWork struct {
	index  int
	hash   [20]byte
	length int
}

type pieceResult struct {
	index int
	buf   []byte
}

// pieceProgress is the state of one piece being downloaded.
type pieceProgress struct {
	index      int
	client     *peer.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

func (p *pieceProgress) readMessage() error {
	msg, err := p.client.Read()
	if err != nil {
		return err
	}
	if msg == nil {
		return nil // keep-alive
	}
	if msg.ID == peer.MsgPiece {
		n, err := peer.ParsePiece(p.index, p.buf, msg)
		if err != nil {
			return err
		}
		p.downloaded += n
		p.backlog--
	}
	// client.Read already handled choke, unchoke and have.
	return nil
}

// downloadPiece fetches one piece from c, keeping up to MaxBacklog block
// requests outstanding.
func downloadPiece(c *peer.Client, pw *pieceWork, timeout time.Duration) ([]byte, error) {
	state := pieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}

	if err := c.Conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	defer c.Conn.SetDeadline(time.Time{})

	for state.downloaded < pw.length {
		if !c.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := BlockSize
				if rem := pw.length - state.requested; rem < blockSize {
					blockSize = rem
				}
				if err := c.SendRequest(pw.index, state.requested, blockSize); err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}
		if err := state.readMessage(); err != nil {
			return nil, err
		}
		if err := c.Conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, err
		}
	}
	return state.buf, nil
}

func checkIntegrity(pw *pieceWork, buf []byte) error {
	got := sha1.Sum(buf)
	if !bytes.Equal(got[:], pw.hash[:]) {
		return fmt.Errorf("piece %d failed integrity check: got %x want %x", pw.index, got, pw.hash)
	}
	return nil
}
