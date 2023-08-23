package trace

import (
	"errors"
	"net"
	"testing"
)

type stubResolver struct {
	names map[string][]string
	err   error
	calls int
}

func (s *stubResolver) LookupAddr(addr string) ([]string, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.names[addr], nil
}

func TestHostResolver_Name(t *testing.T) {
	r := &stubResolver{names: map[string][]string{
		"192.168.1.1": {"router.lan."},
	}}
	h := NewHostResolver(r, false)
	if got := h.Host(net.ParseIP("192.168.1.1")); got != "router.lan" {
		t.Errorf("Host = %q, want router.lan (trailing dot stripped)", got)
	}
}

func TestHostResolver_FallbackToIP(t *testing.T) {
	r := &stubResolver{err: errors.New("no PTR record")}
	h := NewHostResolver(r, false)
	if got := h.Host(net.ParseIP("8.8.8.8")); got != "8.8.8.8" {
		t.Errorf("Host = %q, want bare IP fallback", got)
	}
}

func TestHostResolver_Numeric(t *testing.T) {
	r := &stubResolver{names: map[string][]string{"8.8.8.8": {"dns.google."}}}
	h := NewHostResolver(r, true)
	if got := h.Host(net.ParseIP("8.8.8.8")); got != "8.8.8.8" {
		t.Errorf("Host = %q, want bare IP in numeric mode", got)
	}
	if r.calls != 0 {
		t.Errorf("resolver called %d times in numeric mode, want 0", r.calls)
	}
}

func TestHostResolver_Caches(t *testing.T) {
	r := &stubResolver{names: map[string][]string{"1.1.1.1": {"one.one.one.one."}}}
	h := NewHostResolver(r, false)
	ip := net.ParseIP("1.1.1.1")
	_ = h.Host(ip)
	_ = h.Host(ip)
	if r.calls != 1 {
		t.Errorf("resolver called %d times, want 1 (cached)", r.calls)
	}
}
