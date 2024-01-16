// Package cache implements a TTL-aware DNS answer cache.
//
// Entries hold the raw upstream response and an expiry time. On a hit, every
// record's TTL is rewritten to the time remaining so clients see it count down.
package cache

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dns-forwarder/internal/dnsmsg"
)

// Clock lets tests control time.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Key identifies a cached answer. Name is lowercased and trailing-dot trimmed.
type Key struct {
	Name  string
	Type  uint16
	Class uint16
}

// NormalizeKey returns a canonical Key for the given question fields.
func NormalizeKey(name string, qtype, qclass uint16) Key {
	return Key{
		Name:  strings.ToLower(strings.TrimSuffix(name, ".")),
		Type:  qtype,
		Class: qclass,
	}
}

type entry struct {
	msg       []byte
	expiresAt time.Time
}

// Cache is a concurrency-safe TTL cache. The zero value is not usable; use New.
type Cache struct {
	mu     sync.RWMutex
	m      map[Key]entry
	clock  Clock
	maxTTL uint32 // 0 = no cap
	minTTL uint32 // 0 = no floor

	hits   atomic.Int64
	misses atomic.Int64
	stores atomic.Int64
}

// Options configures a Cache.
type Options struct {
	Clock  Clock  // defaults to wall clock
	MaxTTL uint32 // optional cap on cached TTL seconds (0 = off)
	MinTTL uint32 // optional floor on cached TTL seconds (0 = off)
}

// New returns a Cache using the wall clock and no TTL clamping.
func New() *Cache { return NewWithOptions(Options{}) }

// NewWithOptions returns a configured Cache.
func NewWithOptions(opts Options) *Cache {
	clk := opts.Clock
	if clk == nil {
		clk = realClock{}
	}
	return &Cache{
		m:      make(map[Key]entry),
		clock:  clk,
		maxTTL: opts.MaxTTL,
		minTTL: opts.MinTTL,
	}
}

func (c *Cache) effectiveTTL(ttl uint32) uint32 {
	if c.minTTL > 0 && ttl < c.minTTL {
		ttl = c.minTTL
	}
	if c.maxTTL > 0 && ttl > c.maxTTL {
		ttl = c.maxTTL
	}
	return ttl
}

// Put stores raw response bytes under key with the given base TTL (seconds).
// The bytes are copied. A zero effective TTL is not cached.
func (c *Cache) Put(key Key, raw []byte, ttl uint32) {
	ttl = c.effectiveTTL(ttl)
	if ttl == 0 {
		return
	}
	c.mu.Lock()
	c.m[key] = entry{
		msg:       append([]byte(nil), raw...),
		expiresAt: c.clock.Now().Add(time.Duration(ttl) * time.Second),
	}
	c.mu.Unlock()
	c.stores.Add(1)
}

// Get returns a fresh copy of the cached response for key with all record TTLs
// rewritten to the remaining lifetime. ok is false on a miss or expiry. Expired
// entries are evicted lazily.
func (c *Cache) Get(key Key) (raw []byte, ok bool) {
	now := c.clock.Now()

	c.mu.RLock()
	e, found := c.m[key]
	c.mu.RUnlock()

	if !found || !now.Before(e.expiresAt) {
		if found {
			c.mu.Lock()
			if cur, still := c.m[key]; still && !now.Before(cur.expiresAt) {
				delete(c.m, key)
			}
			c.mu.Unlock()
		}
		c.misses.Add(1)
		return nil, false
	}

	remaining := uint32(e.expiresAt.Sub(now) / time.Second)
	cp := append([]byte(nil), e.msg...)
	if err := dnsmsg.RewriteTTL(cp, remaining); err != nil {
		// Corrupt cache entry; treat as a miss and drop it.
		c.mu.Lock()
		delete(c.m, key)
		c.mu.Unlock()
		c.misses.Add(1)
		return nil, false
	}
	c.hits.Add(1)
	return cp, true
}

// Len returns the number of entries currently held (including not-yet-evicted
// expired ones).
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}

// Stats reports cumulative counters.
func (c *Cache) Stats() (hits, misses, stores int64) {
	return c.hits.Load(), c.misses.Load(), c.stores.Load()
}
