// Package dbm is a single-file, disk-based key/value store in the spirit
// of Unix dbm/gdbm. Keys and values are arbitrary byte strings. Records
// are placed by extendible hashing: a directory of page numbers, indexed
// by the low bits of the key hash, points at bucket pages that split as
// they fill, doubling the directory when a bucket is already as deep as
// the directory. Entries too big for a page continue into overflow pages,
// so there is no limit on key or value size.
package dbm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"ccdbm/internal/hashfn"
)

// Errors returned by the store.
var (
	ErrNotFound = errors.New("dbm: key not found")
	ErrReadOnly = errors.New("dbm: database is read-only")
	ErrClosed   = errors.New("dbm: database is closed")
)

// Options control Create and Open. PageSize and Hash are recorded in the
// file on Create and ignored by Open.
type Options struct {
	PageSize int       // bytes per page; power of two in [MinPageSize, MaxPageSize]; 0 = DefaultPageSize
	Hash     hashfn.ID // hash function; 0 = hashfn.FNV1a
	ReadOnly bool      // open without write access; mutations return ErrReadOnly
	NoSync   bool      // skip fsync after each mutation (faster, less durable)
	Now      func() time.Time
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// DB is an open database. Methods are safe for concurrent use within one
// process; there is no cross-process locking.
type DB struct {
	mu   sync.Mutex
	p    *pager
	path string
	opts Options
	hdr  header
	hash hashfn.Func
	dir  []uint64        // in-memory copy of the directory
	dirt map[uint64]bool // directory pages needing a write
}

// bucket is a bucket loaded into memory.
type bucket struct {
	page    uint64 // head page
	depth   uint8
	entries []entry
	chain   []uint64 // every page of the bucket, head first
}

// Create makes a new, empty database at path. It fails if path exists.
func Create(path string, opts Options) (*DB, error) {
	if opts.PageSize == 0 {
		opts.PageSize = DefaultPageSize
	}
	if opts.PageSize < MinPageSize || opts.PageSize > MaxPageSize || opts.PageSize&(opts.PageSize-1) != 0 {
		return nil, fmt.Errorf("dbm: page size must be a power of two between %d and %d", MinPageSize, MaxPageSize)
	}
	if opts.Hash == 0 {
		opts.Hash = hashfn.FNV1a
	}
	if !opts.Hash.Valid() {
		return nil, fmt.Errorf("dbm: unknown hash function id %d", opts.Hash)
	}
	if opts.ReadOnly {
		return nil, errors.New("dbm: cannot create a read-only database")
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	db := &DB{
		p:    &pager{f: f, pageSize: opts.PageSize, numPages: 1},
		path: path,
		opts: opts,
		hash: opts.Hash.Func(),
		dirt: map[uint64]bool{},
	}
	db.hdr = header{pageSize: uint32(opts.PageSize), hashID: uint32(opts.Hash)}
	// Start with a one-slot directory pointing at a single empty bucket.
	db.hdr.dirPage = db.p.allocRun(1)
	db.hdr.dirPages = 1
	bp, _ := db.p.alloc()
	db.dir = []uint64{bp}
	db.dirt[0] = true
	if err := db.writeBucket(&bucket{page: bp, chain: []uint64{bp}}); err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	if err := db.commit(); err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	return db, nil
}

// Open opens an existing database.
func Open(path string, opts Options) (*DB, error) {
	flag := os.O_RDWR
	if opts.ReadOnly {
		flag = os.O_RDONLY
	}
	f, err := os.OpenFile(path, flag, 0)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, headerSize)
	if _, err := f.ReadAt(buf, 0); err != nil {
		f.Close()
		return nil, corrupt("file too short for a header")
	}
	hdr, err := decodeHeader(buf)
	if err != nil {
		f.Close()
		return nil, err
	}
	if !hashfn.ID(hdr.hashID).Valid() {
		f.Close()
		return nil, corrupt("unknown hash function id %d", hdr.hashID)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if st.Size() < int64(hdr.numPages)*int64(hdr.pageSize) {
		f.Close()
		return nil, corrupt("header claims %d pages but file holds %d bytes", hdr.numPages, st.Size())
	}
	opts.PageSize = int(hdr.pageSize)
	opts.Hash = hashfn.ID(hdr.hashID)
	db := &DB{
		p:    &pager{f: f, pageSize: int(hdr.pageSize), numPages: hdr.numPages, freeHead: hdr.freeHead, readOnly: opts.ReadOnly},
		path: path,
		opts: opts,
		hdr:  hdr,
		hash: opts.Hash.Func(),
		dirt: map[uint64]bool{},
	}
	if err := db.loadDirectory(); err != nil {
		f.Close()
		return nil, err
	}
	return db, nil
}

// Close flushes and closes the file. It is safe to call twice.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.p == nil {
		return nil
	}
	var err error
	if !db.p.readOnly {
		err = db.p.sync()
	}
	if cerr := db.p.f.Close(); err == nil {
		err = cerr
	}
	db.p = nil
	return err
}

// Path returns the file the database lives in.
func (db *DB) Path() string { return db.path }

// PageSize returns the page size recorded in the file.
func (db *DB) PageSize() int { return int(db.hdr.pageSize) }

// HashID returns the hash function recorded in the file.
func (db *DB) HashID() hashfn.ID { return hashfn.ID(db.hdr.hashID) }

// Len returns the number of stored keys, counting expired entries that
// have not yet been purged by a Get, Delete or Reorganise.
func (db *DB) Len() uint64 {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.hdr.numKeys
}

// GlobalDepth returns the number of hash bits the directory indexes on.
func (db *DB) GlobalDepth() int { return int(db.hdr.globalDepth) }

func (db *DB) check() error {
	if db.p == nil {
		return ErrClosed
	}
	return nil
}

func (db *DB) mask() uint64 { return uint64(len(db.dir)) - 1 }

func (db *DB) expired(e entry) bool {
	return e.expires != 0 && uint64(db.opts.now().Unix()) >= e.expires
}

// Put stores value under key, replacing any existing value.
func (db *DB) Put(key, value []byte) error { return db.PutExpires(key, value, 0) }

// PutTTL stores value under key and expires it after ttl (ttl <= 0 means
// no expiry).
func (db *DB) PutTTL(key, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return db.PutExpires(key, value, 0)
	}
	return db.PutExpires(key, value, uint64(db.opts.now().Add(ttl).Unix()))
}

// PutExpires stores value under key with an absolute expiry (Unix seconds,
// 0 = never).
func (db *DB) PutExpires(key, value []byte, expires uint64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.check(); err != nil {
		return err
	}
	if db.p.readOnly {
		return ErrReadOnly
	}
	h := db.hash(key)
	b, err := db.readBucket(db.dir[h&db.mask()])
	if err != nil {
		return err
	}
	existed := false
	for i := range b.entries {
		if bytes.Equal(b.entries[i].key, key) {
			b.entries[i].val = append([]byte{}, value...)
			b.entries[i].expires = expires
			existed = true
			break
		}
	}
	if !existed {
		b.entries = append(b.entries, entry{key: append([]byte{}, key...), val: append([]byte{}, value...), expires: expires})
	}
	payload := db.p.payload()
	for splits := 0; entriesSize(b.entries) > payload && splits < maxSplitsPerInsert && db.splittable(b, h); splits++ {
		other, err := db.split(b, h)
		if err != nil {
			return err
		}
		if err := db.writeBucket(other); err != nil {
			return err
		}
	}
	if err := db.writeBucket(b); err != nil {
		return err
	}
	if !existed {
		db.hdr.numKeys++
	}
	return db.commit()
}

// Get returns the value stored under key.
func (db *DB) Get(key []byte) ([]byte, error) {
	v, _, err := db.GetExpires(key)
	return v, err
}

// GetExpires returns the value stored under key and its expiry (Unix
// seconds, 0 = never).
func (db *DB) GetExpires(key []byte) ([]byte, uint64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.check(); err != nil {
		return nil, 0, err
	}
	b, err := db.readBucket(db.dir[db.hash(key)&db.mask()])
	if err != nil {
		return nil, 0, err
	}
	for _, e := range b.entries {
		if bytes.Equal(e.key, key) {
			if db.expired(e) {
				return nil, 0, ErrNotFound
			}
			return e.val, e.expires, nil
		}
	}
	return nil, 0, ErrNotFound
}

// Delete removes key. An expired entry is removed but reported as
// ErrNotFound.
func (db *DB) Delete(key []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.check(); err != nil {
		return err
	}
	if db.p.readOnly {
		return ErrReadOnly
	}
	b, err := db.readBucket(db.dir[db.hash(key)&db.mask()])
	if err != nil {
		return err
	}
	for i, e := range b.entries {
		if !bytes.Equal(e.key, key) {
			continue
		}
		wasLive := !db.expired(e)
		b.entries = append(b.entries[:i], b.entries[i+1:]...)
		if err := db.writeBucket(b); err != nil {
			return err
		}
		db.hdr.numKeys--
		if err := db.commit(); err != nil {
			return err
		}
		if !wasLive {
			return ErrNotFound
		}
		return nil
	}
	return ErrNotFound
}

// Sync flushes the file to stable storage.
func (db *DB) Sync() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := db.check(); err != nil {
		return err
	}
	return db.p.sync()
}

func (db *DB) readBucket(page uint64) (*bucket, error) {
	b := &bucket{page: page}
	var stream []byte
	var count uint32
	for n := page; n != 0; {
		buf, err := db.p.read(n)
		if err != nil {
			return nil, err
		}
		hdr := decodePageHdr(buf)
		if n == page {
			if hdr.typ != pageBucket {
				return nil, corrupt("directory points at page %d of type %d", n, hdr.typ)
			}
			b.depth = hdr.depth
			count = hdr.count
		} else if hdr.typ != pageOverflow {
			return nil, corrupt("bucket chain reaches page %d of type %d", n, hdr.typ)
		}
		if int(hdr.used) > db.p.payload() {
			return nil, corrupt("page %d claims %d payload bytes", n, hdr.used)
		}
		stream = append(stream, buf[pageHdrSize:pageHdrSize+int(hdr.used)]...)
		b.chain = append(b.chain, n)
		if uint64(len(b.chain)) > db.p.numPages {
			return nil, corrupt("cycle in bucket chain from page %d", page)
		}
		n = hdr.next
	}
	entries, err := decodeEntries(stream, count)
	if err != nil {
		return nil, err
	}
	b.entries = entries
	return b, nil
}

// writeBucket serialises b across its chain, allocating or freeing pages
// so the chain is exactly as long as needed.
func (db *DB) writeBucket(b *bucket) error {
	stream := encodeEntries(b.entries)
	payload := db.p.payload()
	need := (len(stream) + payload - 1) / payload
	if need == 0 {
		need = 1
	}
	for len(b.chain) < need {
		n, err := db.p.alloc()
		if err != nil {
			return err
		}
		b.chain = append(b.chain, n)
	}
	for len(b.chain) > need {
		last := b.chain[len(b.chain)-1]
		if err := db.p.free(last); err != nil {
			return err
		}
		b.chain = b.chain[:len(b.chain)-1]
	}
	buf := make([]byte, db.p.pageSize)
	for i, pg := range b.chain {
		start := i * payload
		end := min(start+payload, len(stream))
		hdr := pageHdr{typ: pageOverflow, depth: b.depth, count: uint32(len(b.entries)), used: uint32(end - start)}
		if i == 0 {
			hdr.typ = pageBucket
		}
		if i+1 < len(b.chain) {
			hdr.next = b.chain[i+1]
		}
		clear(buf)
		hdr.encode(buf)
		copy(buf[pageHdrSize:], stream[start:end])
		if err := db.p.write(pg, buf); err != nil {
			return err
		}
	}
	return nil
}

// splittable reports whether splitting b would separate its entries
// within the next splitWindow hash bits. h is the hash of the key being
// inserted (already in b.entries).
func (db *DB) splittable(b *bucket, h uint64) bool {
	if len(b.entries) < 2 {
		return false
	}
	if uint32(b.depth) >= db.hdr.globalDepth && db.hdr.globalDepth >= maxGlobalDepth {
		return false
	}
	window := uint64(1)<<splitWindow - 1
	ref := (h >> b.depth) & window
	for _, e := range b.entries {
		if (db.hash(e.key)>>b.depth)&window != ref {
			return true
		}
	}
	return false
}

// split divides b (reached by hash h) in two. On return b holds the half
// that h maps to and keeps its pages; the returned bucket is the other
// half on a fresh page. The directory is updated in memory and marked
// dirty; the caller writes both buckets and commits.
func (db *DB) split(b *bucket, h uint64) (*bucket, error) {
	if uint32(b.depth) == db.hdr.globalDepth {
		if err := db.doubleDirectory(); err != nil {
			return nil, err
		}
	}
	bit := uint64(1) << b.depth
	want := h & bit
	np, err := db.p.alloc()
	if err != nil {
		return nil, err
	}
	other := &bucket{page: np, depth: b.depth + 1, chain: []uint64{np}}
	kept := b.entries[:0]
	for _, e := range b.entries {
		if db.hash(e.key)&bit == want {
			kept = append(kept, e)
		} else {
			other.entries = append(other.entries, e)
		}
	}
	b.entries = kept
	b.depth++
	// Directory slots referring to b are canon + k*bit; the ones whose new
	// bit differs from the inserted key's now point at the other bucket.
	canon := h & (bit - 1)
	for i := canon; i < uint64(len(db.dir)); i += bit {
		if i&bit != want {
			db.dir[i] = other.page
			db.dirt[i/db.dirSlotsPerPage()] = true
		}
	}
	return other, nil
}

func (db *DB) dirSlotsPerPage() uint64 { return uint64(db.p.payload() / 8) }

func (db *DB) dirPagesNeeded(slots uint64) uint64 {
	per := db.dirSlotsPerPage()
	return (slots + per - 1) / per
}

func (db *DB) doubleDirectory() error {
	if db.hdr.globalDepth >= maxGlobalDepth {
		return errors.New("dbm: directory is at maximum depth")
	}
	db.dir = append(db.dir, db.dir...)
	db.hdr.globalDepth++
	need := db.dirPagesNeeded(uint64(len(db.dir)))
	if need > uint64(db.hdr.dirPages) {
		old, oldN := db.hdr.dirPage, uint64(db.hdr.dirPages)
		db.hdr.dirPage = db.p.allocRun(need)
		db.hdr.dirPages = uint32(need)
		for i := uint64(0); i < oldN; i++ {
			if err := db.p.free(old + i); err != nil {
				return err
			}
		}
	}
	for i := uint64(0); i < need; i++ {
		db.dirt[i] = true
	}
	return nil
}

func (db *DB) loadDirectory() error {
	slots := uint64(1) << db.hdr.globalDepth
	if need := db.dirPagesNeeded(slots); need > uint64(db.hdr.dirPages) {
		return corrupt("directory needs %d pages but header records %d", need, db.hdr.dirPages)
	}
	db.dir = make([]uint64, 0, slots)
	per := db.dirSlotsPerPage()
	for pg := uint64(0); uint64(len(db.dir)) < slots; pg++ {
		buf, err := db.p.read(db.hdr.dirPage + pg)
		if err != nil {
			return err
		}
		if buf[0] != pageDir {
			return corrupt("directory page %d has type %d", db.hdr.dirPage+pg, buf[0])
		}
		for i := uint64(0); i < per && uint64(len(db.dir)) < slots; i++ {
			v := binary.LittleEndian.Uint64(buf[pageHdrSize+int(i)*8:])
			if v == 0 || v >= db.p.numPages {
				return corrupt("directory slot %d points at page %d", len(db.dir), v)
			}
			db.dir = append(db.dir, v)
		}
	}
	return nil
}

func (db *DB) writeDirPage(pg uint64) error {
	buf := make([]byte, db.p.pageSize)
	pageHdr{typ: pageDir}.encode(buf)
	per := db.dirSlotsPerPage()
	start := pg * per
	for i := uint64(0); i < per && start+i < uint64(len(db.dir)); i++ {
		binary.LittleEndian.PutUint64(buf[pageHdrSize+int(i)*8:], db.dir[start+i])
	}
	return db.p.write(db.hdr.dirPage+pg, buf)
}

// commit writes dirty directory pages and the header, then syncs unless
// NoSync is set. The header goes last so a crash mid-mutation leaves the
// previous header describing the previous (still intact) structure.
func (db *DB) commit() error {
	for pg := range db.dirt {
		if err := db.writeDirPage(pg); err != nil {
			return err
		}
	}
	clear(db.dirt)
	db.hdr.numPages = db.p.numPages
	db.hdr.freeHead = db.p.freeHead
	buf := make([]byte, db.p.pageSize)
	db.hdr.encode(buf)
	if err := db.p.write(0, buf); err != nil {
		return err
	}
	if db.opts.NoSync {
		return nil
	}
	return db.p.sync()
}
