// Package store is the in-memory cache behind the server: items with TTLs,
// CAS tokens, memory accounting and LRU eviction.
//
// Expiry is checked lazily on access. ReapExpired can be called periodically
// to free memory held by expired items nobody reads.
package store

import (
	"container/list"
	"sync"
	"time"
)

// An exptime above thirtyDays is an absolute Unix timestamp, not an offset.
const thirtyDays int64 = 60 * 60 * 24 * 30

// DefaultMaxItemBytes matches memcached's default 1 MiB item size limit.
const DefaultMaxItemBytes int64 = 1 << 20

// itemOverhead is a rough per-item bookkeeping cost added to key and value
// sizes when accounting memory.
const itemOverhead = 64

// Result is the outcome of a store operation.
type Result int

const (
	ResultStored    Result = iota // also used for a successful delete
	ResultNotStored               // add/replace/append/prepend condition failed
	ResultExists                  // cas token didn't match
	ResultNotFound
	ResultTooLarge
)

// Item is a stored value plus its metadata.
type Item struct {
	Key    string
	Flags  uint32
	Value  []byte
	Expiry int64 // absolute Unix seconds; 0 = never expires
	CAS    uint64
	elem   *list.Element // in Store.lru
}

// ItemView is what Get returns for each hit.
type ItemView struct {
	Key   string
	Flags uint32
	Value []byte
	CAS   uint64
}

// Store is safe for concurrent use.
type Store struct {
	mu           sync.RWMutex
	items        map[string]*Item
	lru          *list.List // front = most recently used
	casSeq       uint64
	curBytes     int64
	maxBytes     int64 // memory budget; <= 0 means unlimited
	maxItemBytes int64
	now          func() int64
}

// New returns a Store with a memory budget of maxBytes (0 = unlimited).
func New(maxBytes int64) *Store {
	return NewWithClock(maxBytes, func() int64 { return time.Now().Unix() })
}

// NewWithClock is New with a fake clock, for testing expiry.
func NewWithClock(maxBytes int64, now func() int64) *Store {
	return &Store{
		items:        make(map[string]*Item),
		lru:          list.New(),
		maxBytes:     maxBytes,
		maxItemBytes: DefaultMaxItemBytes,
		now:          now,
	}
}

func itemSize(key string, val []byte) int64 {
	return int64(len(key)) + int64(len(val)) + itemOverhead
}

// normalizeExpiry turns a protocol exptime into an absolute Unix time, or 0
// for "never".
func (s *Store) normalizeExpiry(exptime int64) int64 {
	switch {
	case exptime == 0:
		return 0
	case exptime < 0:
		return s.now() - 1 // expired already
	case exptime <= thirtyDays:
		return s.now() + exptime
	default:
		return exptime
	}
}

func (s *Store) isExpired(it *Item) bool {
	return it.Expiry != 0 && it.Expiry <= s.now()
}

// tooLarge reports whether the item is over the per-item limit or couldn't fit
// in the whole memory budget even if everything else were evicted.
func (s *Store) tooLarge(key string, val []byte) bool {
	return int64(len(val)) > s.maxItemBytes ||
		(s.maxBytes > 0 && itemSize(key, val) > s.maxBytes)
}

// getLiveLocked returns the item for key, dropping it first if it has
// expired. Caller must hold the write lock.
func (s *Store) getLiveLocked(key string) *Item {
	it, ok := s.items[key]
	if !ok {
		return nil
	}
	if s.isExpired(it) {
		s.removeLocked(it)
		return nil
	}
	return it
}

func (s *Store) removeLocked(it *Item) {
	s.lru.Remove(it.elem)
	delete(s.items, it.Key)
	s.curBytes -= itemSize(it.Key, it.Value)
}

func (s *Store) evictToFit(newSize int64) {
	if s.maxBytes <= 0 {
		return
	}
	for s.curBytes+newSize > s.maxBytes {
		back := s.lru.Back()
		if back == nil {
			break
		}
		s.removeLocked(back.Value.(*Item))
	}
}

// putLocked inserts or replaces key with a fresh CAS token. Caller must hold
// the write lock.
func (s *Store) putLocked(key string, flags uint32, expiry int64, value []byte) {
	s.casSeq++
	if old, ok := s.items[key]; ok {
		s.removeLocked(old)
	}
	newSize := itemSize(key, value)
	s.evictToFit(newSize)
	it := &Item{Key: key, Flags: flags, Value: value, Expiry: expiry, CAS: s.casSeq}
	it.elem = s.lru.PushFront(it)
	s.items[key] = it
	s.curBytes += newSize
}

// Set stores key unconditionally.
func (s *Store) Set(key string, flags uint32, exptime int64, value []byte) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tooLarge(key, value) {
		return ResultTooLarge
	}
	s.putLocked(key, flags, s.normalizeExpiry(exptime), value)
	return ResultStored
}

// Add stores key only if it does not already exist.
func (s *Store) Add(key string, flags uint32, exptime int64, value []byte) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tooLarge(key, value) {
		return ResultTooLarge
	}
	if s.getLiveLocked(key) != nil {
		return ResultNotStored
	}
	s.putLocked(key, flags, s.normalizeExpiry(exptime), value)
	return ResultStored
}

// Replace stores key only if it already exists.
func (s *Store) Replace(key string, flags uint32, exptime int64, value []byte) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tooLarge(key, value) {
		return ResultTooLarge
	}
	if s.getLiveLocked(key) == nil {
		return ResultNotStored
	}
	s.putLocked(key, flags, s.normalizeExpiry(exptime), value)
	return ResultStored
}

// Append adds data to the end of an existing item, keeping its flags and
// expiry.
func (s *Store) Append(key string, value []byte) Result {
	return s.concat(key, value, true)
}

// Prepend adds data to the start of an existing item, keeping its flags and
// expiry.
func (s *Store) Prepend(key string, value []byte) Result {
	return s.concat(key, value, false)
}

func (s *Store) concat(key string, value []byte, after bool) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	it := s.getLiveLocked(key)
	if it == nil {
		return ResultNotStored
	}
	joined := make([]byte, 0, len(it.Value)+len(value))
	if after {
		joined = append(append(joined, it.Value...), value...)
	} else {
		joined = append(append(joined, value...), it.Value...)
	}
	if s.tooLarge(key, joined) {
		return ResultTooLarge
	}
	s.putLocked(key, it.Flags, it.Expiry, joined)
	return ResultStored
}

// Cas stores key only if casUnique matches the item's current CAS token.
func (s *Store) Cas(key string, flags uint32, exptime int64, value []byte, casUnique uint64) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tooLarge(key, value) {
		return ResultTooLarge
	}
	it := s.getLiveLocked(key)
	if it == nil {
		return ResultNotFound
	}
	if it.CAS != casUnique {
		return ResultExists
	}
	s.putLocked(key, flags, s.normalizeExpiry(exptime), value)
	return ResultStored
}

// Get returns the live items among keys, in order, marking each as recently
// used. Missing keys are skipped.
func (s *Store) Get(keys []string) []ItemView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ItemView, 0, len(keys))
	for _, k := range keys {
		it := s.getLiveLocked(k)
		if it == nil {
			continue
		}
		s.lru.MoveToFront(it.elem)
		out = append(out, ItemView{Key: it.Key, Flags: it.Flags, Value: it.Value, CAS: it.CAS})
	}
	return out
}

// Delete removes key. It returns ResultStored if the key existed and
// ResultNotFound otherwise.
func (s *Store) Delete(key string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	it := s.getLiveLocked(key)
	if it == nil {
		return ResultNotFound
	}
	s.removeLocked(it)
	return ResultStored
}

// ReapExpired removes all expired items and returns how many there were.
func (s *Store) ReapExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, it := range s.items {
		if s.isExpired(it) {
			s.removeLocked(it) // deleting during range is fine in Go
			n++
		}
	}
	return n
}

// Len returns the number of items held, including expired ones not yet
// removed.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Bytes returns the accounted memory usage.
func (s *Store) Bytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.curBytes
}
