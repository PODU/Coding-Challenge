// Package store is an in-memory key/value store with per-key expiry. A single
// RWMutex guards everything, which is what makes INCR and friends atomic
// across clients.
package store

import (
	"errors"
	"math"
	"strconv"
	"sync"
	"time"
)

var (
	// ErrWrongType means the key holds a different type (e.g. INCR on a list).
	ErrWrongType  = errors.New("wrongtype")
	ErrNotInteger = errors.New("not an integer")
	ErrOverflow   = errors.New("overflow")
)

type entry struct {
	value     any   // string (SET/GET) or []string (LPUSH/RPUSH)
	expiresAt int64 // unix millis; 0 means no expiry
}

// Store is the dataset. now is swappable so tests can control time.
type Store struct {
	mu   sync.RWMutex
	data map[string]entry
	now  func() int64
}

// New returns an empty store that uses the wall clock.
func New() *Store {
	return &Store{
		data: make(map[string]entry),
		now:  func() int64 { return time.Now().UnixMilli() },
	}
}

// SetClock replaces the clock; for tests.
func (s *Store) SetClock(now func() int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

// Now returns the store's clock in unix millis, so EX/PX are computed against
// the same clock the store expires keys with.
func (s *Store) Now() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.now()
}

func isExpired(e entry, now int64) bool {
	return e.expiresAt != 0 && now >= e.expiresAt
}

// SetOptions are the SET modifiers.
type SetOptions struct {
	ExpiresAt int64 // absolute unix millis; 0 = no expiry
	KeepTTL   bool
	NX        bool // only set if absent
	XX        bool // only set if present
}

// Set stores value at key. It returns the previous string value when wantOld
// is set, and ok=false if NX/XX stopped the write. ErrWrongType is only
// possible with wantOld, when the old value isn't a string.
func (s *Store) Set(key, value string, opts SetOptions, wantOld bool) (old string, hadOld bool, ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()

	e, exists := s.data[key]
	if exists && isExpired(e, now) {
		delete(s.data, key)
		exists = false
	}

	if wantOld && exists {
		str, isStr := e.value.(string)
		if !isStr {
			return "", false, false, ErrWrongType
		}
		old, hadOld = str, true
	}

	if (opts.NX && exists) || (opts.XX && !exists) {
		return old, hadOld, false, nil
	}

	ne := entry{value: value}
	if opts.KeepTTL && exists {
		ne.expiresAt = e.expiresAt
	} else {
		ne.expiresAt = opts.ExpiresAt
	}
	s.data[key] = ne
	return old, hadOld, true, nil
}

// Get returns the string at key, deleting it first if it has expired.
func (s *Store) Get(key string) (string, bool, error) {
	s.mu.RLock()
	e, exists := s.data[key]
	now := s.now()
	s.mu.RUnlock()

	if !exists {
		return "", false, nil
	}
	if isExpired(e, now) {
		s.lazyDelete(key)
		return "", false, nil
	}
	str, isStr := e.value.(string)
	if !isStr {
		return "", false, ErrWrongType
	}
	return str, true, nil
}

// lazyDelete re-checks expiry under the write lock, since another client may
// have SET the key again after we dropped the read lock.
func (s *Store) lazyDelete(key string) {
	s.mu.Lock()
	if e, ok := s.data[key]; ok && isExpired(e, s.now()) {
		delete(s.data, key)
	}
	s.mu.Unlock()
}

// Exists counts how many of keys are live.
func (s *Store) Exists(keys ...string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	n := 0
	for _, k := range keys {
		if e, ok := s.data[k]; ok && !isExpired(e, now) {
			n++
		}
	}
	return n
}

// Del removes keys and returns how many were live.
func (s *Store) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	n := 0
	for _, k := range keys {
		if e, ok := s.data[k]; ok {
			delete(s.data, k)
			if !isExpired(e, now) {
				n++
			}
		}
	}
	return n
}

// IncrBy adds delta to the integer at key (missing keys start at 0) and keeps
// any existing TTL.
func (s *Store) IncrBy(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()

	e, exists := s.data[key]
	if exists && isExpired(e, now) {
		delete(s.data, key)
		exists = false
	}

	var cur, exp int64
	if exists {
		str, isStr := e.value.(string)
		if !isStr {
			return 0, ErrWrongType
		}
		n, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return 0, ErrNotInteger
		}
		cur = n
		exp = e.expiresAt
	}

	if (delta > 0 && cur > math.MaxInt64-delta) || (delta < 0 && cur < math.MinInt64-delta) {
		return 0, ErrOverflow
	}
	cur += delta
	s.data[key] = entry{value: strconv.FormatInt(cur, 10), expiresAt: exp}
	return cur, nil
}

// Push adds values to the head (left) or tail of the list at key and returns
// the new length.
func (s *Store) Push(key string, values []string, left bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()

	e, exists := s.data[key]
	if exists && isExpired(e, now) {
		delete(s.data, key)
		exists = false
	}

	var list []string
	var exp int64
	if exists {
		l, isList := e.value.([]string)
		if !isList {
			return 0, ErrWrongType
		}
		list = l
		exp = e.expiresAt
	}

	if left {
		// LPUSH a b c leaves the list as c b a.
		prepended := make([]string, 0, len(values)+len(list))
		for i := len(values) - 1; i >= 0; i-- {
			prepended = append(prepended, values[i])
		}
		list = append(prepended, list...)
	} else {
		list = append(list, values...)
	}
	s.data[key] = entry{value: list, expiresAt: exp}
	return len(list), nil
}

// LRange returns list[start..stop] inclusive; negative indexes count from the
// end.
func (s *Store) LRange(key string, start, stop int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()

	e, exists := s.data[key]
	if !exists || isExpired(e, now) {
		return []string{}, nil
	}
	list, isList := e.value.([]string)
	if !isList {
		return nil, ErrWrongType
	}
	n := len(list)
	if start < 0 {
		start += n
	}
	if stop < 0 {
		stop += n
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n {
		return []string{}, nil
	}
	out := make([]string, stop-start+1)
	copy(out, list[start:stop+1])
	return out, nil
}

// PurgeExpired deletes every expired key and returns how many it removed.
func (s *Store) PurgeExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	n := 0
	for k, e := range s.data {
		if isExpired(e, now) {
			delete(s.data, k)
			n++
		}
	}
	return n
}

// Len returns the number of live keys.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now()
	n := 0
	for _, e := range s.data {
		if !isExpired(e, now) {
			n++
		}
	}
	return n
}
