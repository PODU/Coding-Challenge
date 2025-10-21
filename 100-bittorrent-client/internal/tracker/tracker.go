// Package tracker talks to HTTP trackers (BEP 3), including the compact peer
// list from BEP 23.
package tracker

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"bittorrent/internal/bencode"
	"bittorrent/internal/metainfo"
)

// Peer is one peer address from the tracker.
type Peer struct {
	IP   net.IP
	Port uint16
}

// Addr returns host:port for dialing.
func (p Peer) Addr() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}

// Response is a parsed announce response.
type Response struct {
	Interval int
	Peers    []Peer
}

// BuildAnnounceURL returns the announce URL for t.
func BuildAnnounceURL(t *metainfo.Torrent, peerID [20]byte, port int) (string, error) {
	base, err := url.Parse(t.Announce)
	if err != nil {
		return "", fmt.Errorf("tracker: bad announce URL: %w", err)
	}
	q := url.Values{
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(port)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"left":       []string{strconv.FormatInt(t.Length, 10)},
		"compact":    []string{"1"},
	}
	// The tracker wants the raw 20 hash bytes, percent-encoded, not hex.
	q.Set("info_hash", string(t.InfoHash[:]))
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// Announce sends the announce request and parses the reply.
func Announce(t *metainfo.Torrent, peerID [20]byte, port int, timeout time.Duration) (*Response, error) {
	announceURL, err := BuildAnnounceURL(t, peerID, port)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(announceURL)
	if err != nil {
		return nil, fmt.Errorf("tracker: GET: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("tracker: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker: non-200 status %d: %s", resp.StatusCode, string(body))
	}
	return ParseResponse(body)
}

// ParseResponse decodes a tracker response body. Both the compact and the
// dictionary peer list forms are accepted.
func ParseResponse(body []byte) (*Response, error) {
	decoded, err := bencode.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("tracker: decode response: %w", err)
	}
	dict, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("tracker: response is not a dictionary")
	}
	if reason, ok := dict["failure reason"].(string); ok {
		return nil, fmt.Errorf("tracker: failure reason: %s", reason)
	}

	r := &Response{}
	if iv, ok := dict["interval"].(int64); ok {
		r.Interval = int(iv)
	}

	switch peers := dict["peers"].(type) {
	case string:
		r.Peers, err = parseCompactPeers([]byte(peers))
	case []interface{}:
		r.Peers, err = parseDictPeers(peers)
	default:
		return nil, fmt.Errorf("tracker: response has no usable peers field")
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// parseCompactPeers parses the compact form: 6 bytes per peer, an IPv4
// address then a big-endian port.
func parseCompactPeers(data []byte) ([]Peer, error) {
	const peerSize = 6
	if len(data)%peerSize != 0 {
		return nil, fmt.Errorf("tracker: compact peers length %d not a multiple of %d", len(data), peerSize)
	}
	n := len(data) / peerSize
	peers := make([]Peer, n)
	for i := 0; i < n; i++ {
		off := i * peerSize
		peers[i].IP = net.IPv4(data[off], data[off+1], data[off+2], data[off+3])
		peers[i].Port = binary.BigEndian.Uint16(data[off+4 : off+6])
	}
	return peers, nil
}

// parseDictPeers parses the older list-of-dictionaries form.
func parseDictPeers(list []interface{}) ([]Peer, error) {
	peers := make([]Peer, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("tracker: dictionary peer is not a dict")
		}
		ipStr, _ := m["ip"].(string)
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, fmt.Errorf("tracker: invalid peer ip %q", ipStr)
		}
		portVal, ok := m["port"].(int64)
		if !ok {
			return nil, fmt.Errorf("tracker: peer missing port")
		}
		peers = append(peers, Peer{IP: ip, Port: uint16(portVal)})
	}
	return peers, nil
}
