package tracker

import (
	"net/url"
	"strings"
	"testing"

	"bittorrent/internal/bencode"
	"bittorrent/internal/metainfo"
)

func TestParseCompactPeers(t *testing.T) {
	// Two peers: 1.2.3.4:6881 and 10.0.0.1:6969.
	peers := []byte{1, 2, 3, 4, 0x1a, 0xe1, 10, 0, 0, 1, 0x1b, 0x39}
	body, _ := bencode.Encode(map[string]interface{}{
		"interval": int64(1800),
		"peers":    string(peers),
	})
	resp, err := ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Interval != 1800 {
		t.Errorf("interval = %d", resp.Interval)
	}
	if len(resp.Peers) != 2 {
		t.Fatalf("got %d peers", len(resp.Peers))
	}
	if resp.Peers[0].Addr() != "1.2.3.4:6881" {
		t.Errorf("peer 0 = %s", resp.Peers[0].Addr())
	}
	if resp.Peers[1].Addr() != "10.0.0.1:6969" {
		t.Errorf("peer 1 = %s", resp.Peers[1].Addr())
	}
}

func TestParseFailureReason(t *testing.T) {
	body, _ := bencode.Encode(map[string]interface{}{"failure reason": "torrent not found"})
	if _, err := ParseResponse(body); err == nil || !strings.Contains(err.Error(), "torrent not found") {
		t.Errorf("expected failure reason error, got %v", err)
	}
}

func TestParseBadCompactLength(t *testing.T) {
	body, _ := bencode.Encode(map[string]interface{}{"peers": string([]byte{1, 2, 3})})
	if _, err := ParseResponse(body); err == nil {
		t.Error("expected error for non-multiple-of-6 peers")
	}
}

func TestParseDictionaryPeers(t *testing.T) {
	body, _ := bencode.Encode(map[string]interface{}{
		"peers": []interface{}{
			map[string]interface{}{"ip": "1.2.3.4", "port": int64(6881)},
		},
	})
	resp, err := ParseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Peers) != 1 || resp.Peers[0].Addr() != "1.2.3.4:6881" {
		t.Errorf("dict peers = %+v", resp.Peers)
	}
}

func TestBuildAnnounceURL(t *testing.T) {
	var ih, id [20]byte
	for i := range ih {
		ih[i] = byte(i)
		id[i] = byte('A' + i)
	}
	tr := &metainfo.Torrent{
		Announce: "http://tracker.example.com:6969/announce",
		InfoHash: ih,
		Length:   1024,
	}
	u, err := BuildAnnounceURL(tr, id, 6881)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if got := []byte(q.Get("info_hash")); string(got) != string(ih[:]) {
		t.Errorf("info_hash round-trip failed: %x", got)
	}
	if q.Get("compact") != "1" {
		t.Errorf("compact = %q", q.Get("compact"))
	}
	if q.Get("left") != "1024" {
		t.Errorf("left = %q", q.Get("left"))
	}
	if q.Get("port") != "6881" {
		t.Errorf("port = %q", q.Get("port"))
	}
}
