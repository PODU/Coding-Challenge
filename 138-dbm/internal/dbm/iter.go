package dbm

// Iteration order is directory order, then insertion order within a
// bucket. A bucket of local depth L is referenced by every directory slot
// whose low L bits match, so the slot i that "owns" it (the canonical
// slot) is the smallest such: i < 2^L. Visiting a bucket only from its
// canonical slot yields every key exactly once with no bookkeeping, which
// is what lets NextKey resume from nothing but a key.

// Each calls fn for every live (non-expired) entry until fn returns
// false. fn receives the entry's expiry (Unix seconds, 0 = never). The
// database must not be modified from within fn.
func (db *DB) Each(fn func(key, value []byte, expires uint64) bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.check(); err != nil {
		return err
	}
	for i := uint64(0); i < uint64(len(db.dir)); i++ {
		b, ok, err := db.canonicalBucket(i)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		for _, e := range b.entries {
			if db.expired(e) {
				continue
			}
			if !fn(e.key, e.val, e.expires) {
				return nil
			}
		}
	}
	return nil
}

// Keys returns every live key.
func (db *DB) Keys() ([][]byte, error) {
	var out [][]byte
	err := db.Each(func(k, _ []byte, _ uint64) bool {
		out = append(out, k)
		return true
	})
	return out, err
}

// FirstKey returns the first key in iteration order, or nil if the
// database is empty.
func (db *DB) FirstKey() ([]byte, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.check(); err != nil {
		return nil, err
	}
	return db.firstKeyFrom(0)
}

// NextKey returns the key that follows key in iteration order, or nil
// after the last one. If key has been deleted, iteration resumes at the
// start of the bucket it belonged to, so the classic loop
//
//	k := FirstKey(); for k != nil { Delete(k); k = NextKey(k) }
//
// still visits every key.
func (db *DB) NextKey(key []byte) ([]byte, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.check(); err != nil {
		return nil, err
	}
	idx := db.hash(key) & db.mask()
	b, err := db.readBucket(db.dir[idx])
	if err != nil {
		return nil, err
	}
	start := 0
	for i, e := range b.entries {
		if string(e.key) == string(key) {
			start = i + 1
			break
		}
	}
	for _, e := range b.entries[start:] {
		if !db.expired(e) {
			return e.key, nil
		}
	}
	canon := idx & (uint64(1)<<b.depth - 1)
	return db.firstKeyFrom(canon + 1)
}

func (db *DB) firstKeyFrom(i uint64) ([]byte, error) {
	for ; i < uint64(len(db.dir)); i++ {
		b, ok, err := db.canonicalBucket(i)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		for _, e := range b.entries {
			if !db.expired(e) {
				return e.key, nil
			}
		}
	}
	return nil, nil
}

// canonicalBucket loads the bucket at directory slot i if i is that
// bucket's canonical slot.
func (db *DB) canonicalBucket(i uint64) (*bucket, bool, error) {
	hdr, err := db.p.readHdr(db.dir[i])
	if err != nil {
		return nil, false, err
	}
	if hdr.typ != pageBucket {
		return nil, false, corrupt("directory slot %d points at page %d of type %d", i, db.dir[i], hdr.typ)
	}
	if i >= uint64(1)<<hdr.depth {
		return nil, false, nil
	}
	b, err := db.readBucket(db.dir[i])
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}
