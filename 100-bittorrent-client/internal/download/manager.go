// Package download fetches pieces from many peers at once, checks their
// hashes and writes them into the output file.
package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"bittorrent/internal/metainfo"
	"bittorrent/internal/peer"
	"bittorrent/internal/tracker"
)

// PeerID is the 20-byte id we send to peers and trackers.
type PeerID [20]byte

// Dialer connects to a peer and completes the handshake. Tests swap in a fake.
type Dialer func(addr string, infoHash, peerID [20]byte, timeout time.Duration) (*peer.Client, error)

// Manager owns a single torrent download.
type Manager struct {
	Torrent  *metainfo.Torrent
	PeerID   [20]byte
	Timeout  time.Duration
	MaxPeers int
	Verbose  bool

	Dial   Dialer      // nil means peer.NewClient
	Logger *log.Logger // nil means the standard logger
}

func (m *Manager) logf(format string, args ...interface{}) {
	if !m.Verbose {
		return
	}
	if m.Logger != nil {
		m.Logger.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Run downloads every piece from peers and writes each one at its offset in
// out. It returns once all pieces are written, ctx is cancelled, or every peer
// has dropped out.
func (m *Manager) Run(ctx context.Context, peers []tracker.Peer, out io.WriterAt) error {
	if m.Dial == nil {
		m.Dial = peer.NewClient
	}
	if m.Timeout == 0 {
		m.Timeout = 15 * time.Second
	}

	t := m.Torrent
	numPieces := t.NumPieces()
	if numPieces == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workQueue := make(chan *pieceWork, numPieces)
	results := make(chan *pieceResult, numPieces)
	for i := 0; i < numPieces; i++ {
		workQueue <- &pieceWork{
			index:  i,
			hash:   t.PieceHashes[i],
			length: int(t.PieceSize(i)),
		}
	}

	if m.MaxPeers > 0 && len(peers) > m.MaxPeers {
		peers = peers[:m.MaxPeers]
	}

	var wg sync.WaitGroup
	for _, p := range peers {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			m.worker(ctx, addr, workQueue, results)
		}(p.Addr())
	}

	// If every worker gives up, fail instead of waiting forever on results.
	workersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(workersDone)
	}()

	collected := 0
	writePiece := func(res *pieceResult) error {
		if _, err := out.WriteAt(res.buf, int64(res.index)*t.PieceLength); err != nil {
			return fmt.Errorf("download: write piece %d: %w", res.index, err)
		}
		collected++
		return nil
	}
	for collected < numPieces {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res := <-results:
			if err := writePiece(res); err != nil {
				return err
			}
			m.logf("piece %d/%d written (%.1f%%)", collected, numPieces, float64(collected)/float64(numPieces)*100)
		case <-workersDone:
			// Pick up anything the workers delivered before they quit.
			for collected < numPieces {
				select {
				case res := <-results:
					if err := writePiece(res); err != nil {
						return err
					}
				default:
					return fmt.Errorf("download: swarm exhausted with %d/%d pieces complete", collected, numPieces)
				}
			}
		}
	}
	return nil
}

// worker downloads pieces from one peer until ctx is done or the connection
// fails.
func (m *Manager) worker(ctx context.Context, addr string, workQueue chan *pieceWork, results chan<- *pieceResult) {
	c, err := m.Dial(addr, m.Torrent.InfoHash, m.PeerID, m.Timeout)
	if err != nil {
		m.logf("peer %s: connect failed: %v", addr, err)
		return
	}
	defer c.Close()

	if err := c.SendUnchoke(); err != nil {
		m.logf("peer %s: unchoke failed: %v", addr, err)
		return
	}
	if err := c.SendInterested(); err != nil {
		m.logf("peer %s: interested failed: %v", addr, err)
		return
	}
	m.logf("peer %s: connected", addr)

	for {
		select {
		case <-ctx.Done():
			return
		case pw, ok := <-workQueue:
			if !ok {
				return
			}
			if !c.HasPiece(pw.index) {
				// Hand it back for another peer, and sleep a little so we don't
				// spin on a queue full of pieces this peer lacks.
				workQueue <- pw
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
				continue
			}
			buf, err := downloadPiece(c, pw, m.Timeout)
			if err != nil {
				m.logf("peer %s: piece %d download error: %v", addr, pw.index, err)
				workQueue <- pw
				return // don't trust this connection any more
			}
			if err := checkIntegrity(pw, buf); err != nil {
				m.logf("peer %s: %v", addr, err)
				workQueue <- pw
				continue
			}
			if err := c.SendHave(pw.index); err != nil {
				m.logf("peer %s: have failed: %v", addr, err)
			}
			select {
			case results <- &pieceResult{index: pw.index, buf: buf}:
			case <-ctx.Done():
				return
			}
		}
	}
}
