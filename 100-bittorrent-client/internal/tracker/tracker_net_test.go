//go:build net

// These tests perform real network I/O against a live tracker and are excluded
// from the default build. Run them explicitly with:
//
//	go test -tags net ./internal/tracker/ -net -torrent path/to/file.torrent
package tracker

import (
	"flag"
	"os"
	"testing"
	"time"

	"bittorrent/internal/metainfo"
)

var (
	netFlag     = flag.Bool("net", false, "enable live network tests")
	torrentPath = flag.String("torrent", "", "path to a .torrent file for live announce")
)

func TestLiveAnnounce(t *testing.T) {
	if !*netFlag {
		t.Skip("live network tests disabled; pass -net to enable")
	}
	if *torrentPath == "" {
		t.Skip("no -torrent provided")
	}
	data, err := os.ReadFile(*torrentPath)
	if err != nil {
		t.Fatalf("read torrent: %v", err)
	}
	tr, err := metainfo.Parse(data)
	if err != nil {
		t.Fatalf("parse torrent: %v", err)
	}
	var peerID [20]byte
	copy(peerID[:], "-GO0001-livelivelive")

	resp, err := Announce(tr, peerID, 6881, 20*time.Second)
	if err != nil {
		t.Fatalf("announce: %v", err)
	}
	t.Logf("interval=%d peers=%d", resp.Interval, len(resp.Peers))
}
