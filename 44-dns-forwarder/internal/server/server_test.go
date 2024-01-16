package server

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"dns-forwarder/internal/blocklist"
	"dns-forwarder/internal/cache"
	"dns-forwarder/internal/dnsmsg"
)

// fakeExchanger answers every query with a canned A record and counts calls.
type fakeExchanger struct {
	calls atomic.Int64
	ttl   uint32
}

func (f *fakeExchanger) Exchange(_ context.Context, query []byte) ([]byte, error) {
	f.calls.Add(1)
	m, err := dnsmsg.Decode(query)
	if err != nil {
		return nil, err
	}
	q := m.Questions[0]
	resp := &dnsmsg.Message{
		Header:    dnsmsg.Header{ID: m.Header.ID, QR: true, RD: m.Header.RD, RA: true, RCode: dnsmsg.RCodeNoError},
		Questions: []dnsmsg.Question{q},
		Answers: []dnsmsg.ResourceRecord{
			{Name: q.Name, Type: dnsmsg.TypeA, Class: dnsmsg.ClassIN, TTL: f.ttl, RData: []byte{1, 2, 3, 4}},
		},
	}
	return dnsmsg.Encode(resp)
}

func query(id uint16, name string, qtype uint16) []byte {
	m := &dnsmsg.Message{
		Header:    dnsmsg.Header{ID: id, RD: true},
		Questions: []dnsmsg.Question{{Name: name, Type: qtype, Class: dnsmsg.ClassIN}},
	}
	b, _ := dnsmsg.Encode(m)
	return b
}

// newTestServer starts a server with the given deps on an ephemeral port and
// returns it plus a connected UDP client. Cleanup is registered via t.Cleanup.
func newTestServer(t *testing.T, up Exchanger, c *cache.Cache, bl *blocklist.List, sinkhole string) (*Server, *net.UDPConn) {
	t.Helper()
	s := New(Config{Listen: "127.0.0.1:0", Timeout: time.Second, Sinkhole: sinkhole}, up, c, bl)
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})

	raddr := s.LocalAddr().(*net.UDPAddr)
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return s, conn
}

func roundtrip(t *testing.T, conn *net.UDPConn, q []byte) *dnsmsg.Message {
	t.Helper()
	if _, err := conn.Write(q); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 65535)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	m, err := dnsmsg.Decode(buf[:n])
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return m
}

func TestForwardThenCacheHit(t *testing.T) {
	fake := &fakeExchanger{ttl: 300}
	c := cache.New()
	_, conn := newTestServer(t, fake, c, nil, "nxdomain")

	// First query: miss -> forward.
	m1 := roundtrip(t, conn, query(0x0001, "example.com", dnsmsg.TypeA))
	if m1.Header.ID != 0x0001 || len(m1.Answers) != 1 {
		t.Fatalf("first response bad: id=%#x answers=%d", m1.Header.ID, len(m1.Answers))
	}
	if !bytesEqual(m1.Answers[0].RData, []byte{1, 2, 3, 4}) {
		t.Errorf("rdata = %v", m1.Answers[0].RData)
	}

	// Second identical query: served from cache, no extra upstream call.
	m2 := roundtrip(t, conn, query(0x0002, "example.com", dnsmsg.TypeA))
	if m2.Header.ID != 0x0002 {
		t.Errorf("cache hit ID not rewritten: %#x", m2.Header.ID)
	}
	if got := fake.calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1 (second served from cache)", got)
	}
}

func TestDifferentQTypeNotShared(t *testing.T) {
	fake := &fakeExchanger{ttl: 300}
	c := cache.New()
	_, conn := newTestServer(t, fake, c, nil, "nxdomain")

	roundtrip(t, conn, query(1, "example.com", dnsmsg.TypeA))
	roundtrip(t, conn, query(2, "example.com", dnsmsg.TypeAAAA))
	if got := fake.calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2 (A and AAAA cached separately)", got)
	}
}

func TestBlocklistNXDOMAIN(t *testing.T) {
	fake := &fakeExchanger{ttl: 300}
	bl := blocklist.New([]string{"ads.example.com"})
	_, conn := newTestServer(t, fake, cache.New(), bl, "nxdomain")

	m := roundtrip(t, conn, query(0x0042, "ads.example.com", dnsmsg.TypeA))
	if m.Header.RCode != dnsmsg.RCodeNXDomain {
		t.Errorf("RCode = %d, want NXDOMAIN(%d)", m.Header.RCode, dnsmsg.RCodeNXDomain)
	}
	if fake.calls.Load() != 0 {
		t.Errorf("blocked query must not forward, calls=%d", fake.calls.Load())
	}
}

func TestBlocklistSinkholeZero(t *testing.T) {
	fake := &fakeExchanger{ttl: 300}
	bl := blocklist.New([]string{"ads.example.com"})
	_, conn := newTestServer(t, fake, cache.New(), bl, "zero")

	m := roundtrip(t, conn, query(0x0043, "ads.example.com", dnsmsg.TypeA))
	if m.Header.RCode != dnsmsg.RCodeNoError || len(m.Answers) != 1 {
		t.Fatalf("sinkhole response bad: rcode=%d answers=%d", m.Header.RCode, len(m.Answers))
	}
	if !bytesEqual(m.Answers[0].RData, []byte{0, 0, 0, 0}) {
		t.Errorf("sinkhole rdata = %v, want 0.0.0.0", m.Answers[0].RData)
	}
}

func TestMalformedYieldsFormErr(t *testing.T) {
	fake := &fakeExchanger{ttl: 300}
	_, conn := newTestServer(t, fake, cache.New(), nil, "nxdomain")

	// 12-byte header claiming one question but no question body.
	bad := make([]byte, 12)
	binary.BigEndian.PutUint16(bad[0:2], 0x00FF)
	binary.BigEndian.PutUint16(bad[4:6], 1)
	if _, err := conn.Write(bad); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	m, err := dnsmsg.Decode(buf[:n])
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.Header.RCode != dnsmsg.RCodeFormErr {
		t.Errorf("RCode = %d, want FORMERR(%d)", m.Header.RCode, dnsmsg.RCodeFormErr)
	}

	// Server must still be alive: a valid query still resolves.
	good := roundtrip(t, conn, query(0x0100, "example.com", dnsmsg.TypeA))
	if len(good.Answers) != 1 {
		t.Errorf("server not alive after malformed packet")
	}
}

func TestUpstreamFailureServFail(t *testing.T) {
	_, conn := newTestServer(t, failingExchanger{}, cache.New(), nil, "nxdomain")
	m := roundtrip(t, conn, query(0x0200, "example.com", dnsmsg.TypeA))
	if m.Header.RCode != dnsmsg.RCodeServFail {
		t.Errorf("RCode = %d, want SERVFAIL(%d)", m.Header.RCode, dnsmsg.RCodeServFail)
	}
}

type failingExchanger struct{}

func (failingExchanger) Exchange(context.Context, []byte) ([]byte, error) {
	return nil, context.DeadlineExceeded
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
