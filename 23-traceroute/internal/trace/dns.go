package trace

import (
	"net"
	"strings"
	"sync"
)

// Resolver does reverse DNS lookups. Tests replace it with a fake.
type Resolver interface {
	LookupAddr(addr string) ([]string, error)
}

type netResolver struct{}

func (netResolver) LookupAddr(addr string) ([]string, error) {
	return net.LookupAddr(addr)
}

// DefaultResolver uses net.LookupAddr.
func DefaultResolver() Resolver { return netResolver{} }

// HostResolver turns hop IPs into hostnames, falling back to the IP itself.
// Results are cached for the run, and nothing is looked up in numeric mode.
type HostResolver struct {
	r       Resolver
	numeric bool
	mu      sync.Mutex
	cache   map[string]string
}

// NewHostResolver returns a HostResolver. In numeric mode it never does a lookup.
func NewHostResolver(r Resolver, numeric bool) *HostResolver {
	return &HostResolver{
		r:       r,
		numeric: numeric,
		cache:   make(map[string]string),
	}
}

// Host returns the hostname for ip, or ip itself if there is none.
func (h *HostResolver) Host(ip net.IP) string {
	s := ip.String()
	if h.numeric || h.r == nil {
		return s
	}

	h.mu.Lock()
	if v, ok := h.cache[s]; ok {
		h.mu.Unlock()
		return v
	}
	h.mu.Unlock()

	name := s
	if names, err := h.r.LookupAddr(s); err == nil && len(names) > 0 {
		name = strings.TrimSuffix(names[0], ".")
	}

	h.mu.Lock()
	h.cache[s] = name
	h.mu.Unlock()
	return name
}
