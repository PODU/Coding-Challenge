// Package bitcask is a log-structured key-value store following the Bitcask
// paper: append-only data files of CRC-checked records, an in-memory keydir so
// each read is a single seek, tombstone deletes, file rotation, merging with
// hint files, and recovery that drops a torn record at the end of a file.
package bitcask

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMaxFileSize is the active-file rotation threshold (1 GiB).
const DefaultMaxFileSize int64 = 1 << 30

var (
	ErrKeyNotFound    = errors.New("bitcask: key not found")
	ErrEmptyKey       = errors.New("bitcask: empty key")
	ErrChecksum       = errors.New("bitcask: crc mismatch")
	ErrDatabaseLocked = errors.New("bitcask: database already locked by another process")
	ErrReadOnly       = errors.New("bitcask: database opened read-only")
	ErrClosed         = errors.New("bitcask: database closed")
)

// Options configures a Bitcask database.
type Options struct {
	Dir          string // database directory
	MaxFileSize  int64  // active-file rotation threshold (default DefaultMaxFileSize)
	SyncOnPut    bool   // fsync after every write (durable but slower)
	VerifyOnRead bool   // recompute CRC on each Get (defense in depth)
	ReadOnly     bool   // open without acquiring the write lock
}

// Entry is an in-memory keydir record locating the latest value of a key.
type Entry struct {
	FileID    uint32 // which *.cask file holds the record
	ValuePos  int64  // byte offset of the record start in that file
	ValueSize uint32 // value length
	RecSize   uint32 // total record length (header+key+value)
	Timestamp int64  // write time (ns); used for newest-wins resolution
}

// Bitcask is a handle to an open database. It is safe for concurrent use:
// many readers proceed in parallel; writes are serialized.
type Bitcask struct {
	opts Options

	mu     sync.RWMutex     // guards keydir
	keydir map[string]Entry // key -> latest location

	writeMu sync.Mutex  // serializes appends (single writer)
	active  *activeFile // current append target (nil when read-only & empty)

	readersMu sync.Mutex          // guards readers
	readers   map[uint32]*os.File // cached read handles by file id

	mergeMu sync.Mutex // at most one merge at a time

	nextID   atomic.Uint32 // monotonic file-id allocator (highest allocated)
	lockFile *os.File
	closed   atomic.Bool // set once by Close; read lock-free on the Get hot path
}

// Open opens or creates the database at opts.Dir, rebuilding the keydir from
// disk. Unless ReadOnly, it acquires an exclusive directory lock.
func Open(opts Options) (*Bitcask, error) {
	if opts.Dir == "" {
		return nil, errors.New("bitcask: empty Dir")
	}
	if opts.MaxFileSize <= 0 {
		opts.MaxFileSize = DefaultMaxFileSize
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, err
	}

	b := &Bitcask{
		opts:    opts,
		keydir:  make(map[string]Entry),
		readers: make(map[uint32]*os.File),
	}

	if !opts.ReadOnly {
		lf, err := acquireLock(opts.Dir)
		if err != nil {
			return nil, err
		}
		b.lockFile = lf
	}

	if err := b.recover(); err != nil {
		releaseLock(b.lockFile)
		return nil, err
	}
	return b, nil
}

// allocFileID returns an unused file id. Rotation and merge both use it so
// their ids never collide.
func (b *Bitcask) allocFileID() uint32 { return b.nextID.Add(1) }

// Get returns the current value for key, or ErrKeyNotFound.
func (b *Bitcask) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	}
	if b.closed.Load() {
		// Guard against use-after-close: without this a racing Get would call
		// readerFor and reopen a file handle into the fresh readers map that
		// Close just installed, leaking a handle the DB can no longer close.
		return nil, ErrClosed
	}
	b.mu.RLock()
	e, ok := b.keydir[string(key)]
	b.mu.RUnlock()
	if !ok {
		return nil, ErrKeyNotFound
	}

	buf := make([]byte, e.RecSize)
	f, err := b.readerFor(e.FileID)
	if err != nil {
		return nil, err
	}
	if _, err := f.ReadAt(buf, e.ValuePos); err != nil {
		return nil, err
	}
	if b.opts.VerifyOnRead && !verifyCRC(buf) {
		return nil, ErrChecksum
	}
	h := decodeHeader(buf)
	if h.isTombstone() {
		return nil, ErrKeyNotFound
	}
	// Guard against a header whose sizes don't fit the record: with VerifyOnRead
	// off a corrupt (bit-rotted) header is not caught above, and slicing on its
	// raw key/value sizes would panic. Bound-check against the buffer instead.
	valStart := headerSize + int(h.keySize)
	valEnd := valStart + int(h.valueSize)
	if valStart < headerSize || valEnd > len(buf) {
		return nil, ErrChecksum
	}
	value := make([]byte, h.valueSize)
	copy(value, buf[valStart:valEnd])
	return value, nil
}

func (b *Bitcask) Put(key, value []byte) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}
	if b.opts.ReadOnly {
		return ErrReadOnly
	}
	return b.append(key, value, 0)
}

// Delete appends a tombstone for key (if present) and removes it from the
// keydir. Deleting an absent key is a no-op.
func (b *Bitcask) Delete(key []byte) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}
	if b.opts.ReadOnly {
		return ErrReadOnly
	}
	b.mu.RLock()
	_, ok := b.keydir[string(key)]
	b.mu.RUnlock()
	if !ok {
		return nil // idempotent: nothing live to delete
	}
	return b.append(key, nil, flagTombstone)
}

// append writes one record, rotating the active file first if needed, and
// updates the keydir.
func (b *Bitcask) append(key, value []byte, flags byte) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	if b.closed.Load() {
		return ErrClosed
	}
	if b.active == nil {
		return ErrReadOnly
	}

	ts := time.Now().UnixNano()
	rec := encodeRecord(ts, flags, key, value)

	if err := b.maybeRotate(int64(len(rec))); err != nil {
		return err
	}

	pos := b.active.offset
	n, err := b.active.f.Write(rec)
	if err != nil {
		return err
	}
	b.active.offset += int64(n)

	if b.opts.SyncOnPut {
		if err := b.active.f.Sync(); err != nil {
			return err
		}
	}

	b.mu.Lock()
	if flags&flagTombstone != 0 {
		delete(b.keydir, string(key))
	} else {
		b.keydir[string(key)] = Entry{
			FileID:    b.active.id,
			ValuePos:  pos,
			ValueSize: uint32(len(value)),
			RecSize:   uint32(len(rec)),
			Timestamp: ts,
		}
	}
	b.mu.Unlock()
	return nil
}

// maybeRotate closes the active file and opens a fresh one when the incoming
// record would push it past MaxFileSize. An empty active file always accepts at
// least one record, so a value larger than the limit gets its own file.
func (b *Bitcask) maybeRotate(recSize int64) error {
	if b.active.offset == 0 {
		return nil
	}
	if b.active.offset+recSize <= b.opts.MaxFileSize {
		return nil
	}
	if err := b.active.f.Sync(); err != nil {
		return err
	}
	if err := b.active.f.Close(); err != nil {
		return err
	}
	return b.createActive(b.allocFileID())
}

// readerFor returns a cached read-only handle for a file id, opening it on
// first use. ReadAt on the returned handle is safe for concurrent use.
func (b *Bitcask) readerFor(id uint32) (*os.File, error) {
	b.readersMu.Lock()
	defer b.readersMu.Unlock()
	if f, ok := b.readers[id]; ok {
		return f, nil
	}
	f, err := os.Open(filepath.Join(b.opts.Dir, dataFileName(id)))
	if err != nil {
		return nil, err
	}
	b.readers[id] = f
	return f, nil
}

// closeReader drops any cached read handle for a file id. Required on Windows
// before deleting a file that may have open handles (during merge).
func (b *Bitcask) closeReader(id uint32) {
	b.readersMu.Lock()
	if f, ok := b.readers[id]; ok {
		_ = f.Close()
		delete(b.readers, id)
	}
	b.readersMu.Unlock()
}

func (b *Bitcask) readRecord(e Entry) ([]byte, error) {
	buf := make([]byte, e.RecSize)
	f, err := b.readerFor(e.FileID)
	if err != nil {
		return nil, err
	}
	if _, err := f.ReadAt(buf, e.ValuePos); err != nil {
		return nil, err
	}
	if !verifyCRC(buf) {
		return nil, ErrChecksum
	}
	return buf, nil
}

// Has reports whether key currently has a live value.
func (b *Bitcask) Has(key []byte) bool {
	if len(key) == 0 {
		return false
	}
	b.mu.RLock()
	_, ok := b.keydir[string(key)]
	b.mu.RUnlock()
	return ok
}

// Keys returns a snapshot of the live keys.
func (b *Bitcask) Keys() [][]byte {
	b.mu.RLock()
	keys := make([][]byte, 0, len(b.keydir))
	for k := range b.keydir {
		keys = append(keys, []byte(k))
	}
	b.mu.RUnlock()
	return keys
}

// Fold iterates all live key/value pairs, calling fn for each. It snapshots the
// key set first, so it does not hold the keydir lock during value reads.
func (b *Bitcask) Fold(fn func(k, v []byte) error) error {
	b.mu.RLock()
	keys := make([]string, 0, len(b.keydir))
	for k := range b.keydir {
		keys = append(keys, k)
	}
	b.mu.RUnlock()

	for _, k := range keys {
		v, err := b.Get([]byte(k))
		if err == ErrKeyNotFound {
			continue // concurrently deleted
		}
		if err != nil {
			return err
		}
		if err := fn([]byte(k), v); err != nil {
			return err
		}
	}
	return nil
}

// Sync fsyncs the active file.
func (b *Bitcask) Sync() error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	if b.active == nil {
		return nil
	}
	return b.active.f.Sync()
}

// Close fsyncs the active file, closes all handles and releases the directory
// lock. Calling it again is a no-op.
func (b *Bitcask) Close() error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	if b.closed.Load() {
		return nil
	}
	b.closed.Store(true)

	var firstErr error
	if b.active != nil {
		if err := b.active.f.Sync(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := b.active.f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		b.active = nil
	}

	b.readersMu.Lock()
	for _, f := range b.readers {
		_ = f.Close()
	}
	b.readers = make(map[uint32]*os.File)
	b.readersMu.Unlock()

	releaseLock(b.lockFile)
	b.lockFile = nil
	return firstErr
}
