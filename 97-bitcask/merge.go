package bitcask

import (
	"os"
	"path/filepath"
)

// Merge rewrites the live values from the immutable data files into new files
// with hint files, then deletes the old files. The active file is left alone,
// and reads and writes carry on while it runs.
//
// New files are fsynced before any old file is removed, so a crash part-way
// leaves the old files intact. A keydir entry is only repointed if no write
// replaced it during the merge, so a concurrent Put is never lost.
func (b *Bitcask) Merge() error {
	b.mergeMu.Lock()
	defer b.mergeMu.Unlock()
	if b.opts.ReadOnly {
		return ErrReadOnly
	}

	b.writeMu.Lock()
	if b.closed.Load() || b.active == nil {
		b.writeMu.Unlock()
		return ErrClosed
	}
	activeID := b.active.id
	b.writeMu.Unlock()

	// Snapshot live entries that live in immutable files (id < activeID).
	type item struct {
		key string
		e   Entry
	}
	b.mu.RLock()
	var items []item
	for k, e := range b.keydir {
		if e.FileID < activeID {
			items = append(items, item{k, e})
		}
	}
	b.mu.RUnlock()

	// Which immutable files exist and may be removed once compacted.
	ids, err := listDataFileIDs(b.opts.Dir)
	if err != nil {
		return err
	}
	var mergedFileIDs []uint32
	for _, id := range ids {
		if id < activeID {
			mergedFileIDs = append(mergedFileIDs, id)
		}
	}
	if len(mergedFileIDs) == 0 {
		return nil // nothing immutable to merge
	}

	// Write live values into fresh output files (+ hint sidecars).
	mw := newMergeWriter(b)
	type pending struct {
		key      string
		old, new Entry
	}
	var pends []pending
	for _, it := range items {
		rec, err := b.readRecord(it.e)
		if err != nil {
			mw.abort()
			return err
		}
		h := decodeHeader(rec)
		if h.isTombstone() { // defensive; tombstones aren't in the keydir
			continue
		}
		value := rec[headerSize+int(h.keySize) : headerSize+int(h.keySize)+int(h.valueSize)]
		newEntry, err := mw.append(it.e.Timestamp, []byte(it.key), value)
		if err != nil {
			mw.abort()
			return err
		}
		pends = append(pends, pending{it.key, it.e, newEntry})
	}
	if err := mw.finish(); err != nil { // fsync + close data and hint files
		return err
	}

	// Atomic swap: repoint keydir entries that were not superseded meanwhile.
	b.mu.Lock()
	for _, p := range pends {
		cur, ok := b.keydir[p.key]
		if ok && cur.FileID == p.old.FileID && cur.ValuePos == p.old.ValuePos && cur.Timestamp == p.old.Timestamp {
			b.keydir[p.key] = p.new
		}
	}
	b.mu.Unlock()

	// Old files are durable-superseded now; close handles and unlink them.
	for _, id := range mergedFileIDs {
		b.closeReader(id)
		_ = os.Remove(filepath.Join(b.opts.Dir, dataFileName(id)))
		_ = os.Remove(filepath.Join(b.opts.Dir, hintFileName(id)))
	}
	return nil
}

// mergeWriter writes compacted records to one or more output files, rotating at
// MaxFileSize, each paired with a hint file.
type mergeWriter struct {
	b       *Bitcask
	maxSize int64

	curID  uint32
	data   *os.File
	hint   *os.File
	offset int64
}

func newMergeWriter(b *Bitcask) *mergeWriter {
	return &mergeWriter{b: b, maxSize: b.opts.MaxFileSize}
}

func (m *mergeWriter) openNew() error {
	id := m.b.allocFileID()
	dpath := filepath.Join(m.b.opts.Dir, dataFileName(id))
	hpath := filepath.Join(m.b.opts.Dir, hintFileName(id))
	df, err := os.OpenFile(dpath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	hf, err := os.OpenFile(hpath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		df.Close()
		return err
	}
	m.curID, m.data, m.hint, m.offset = id, df, hf, 0
	return nil
}

func (m *mergeWriter) closeCur() error {
	if m.data == nil {
		return nil
	}
	if err := m.data.Sync(); err != nil {
		return err
	}
	if err := m.hint.Sync(); err != nil {
		return err
	}
	if err := m.data.Close(); err != nil {
		return err
	}
	if err := m.hint.Close(); err != nil {
		return err
	}
	m.data, m.hint = nil, nil
	return nil
}

// append writes one record and its hint, rotating output if needed.
func (m *mergeWriter) append(ts int64, key, value []byte) (Entry, error) {
	rec := encodeRecord(ts, 0, key, value)
	recSize := int64(len(rec))

	if m.data != nil && m.offset > 0 && m.offset+recSize > m.maxSize {
		if err := m.closeCur(); err != nil {
			return Entry{}, err
		}
	}
	if m.data == nil {
		if err := m.openNew(); err != nil {
			return Entry{}, err
		}
	}

	pos := m.offset
	if _, err := m.data.Write(rec); err != nil {
		return Entry{}, err
	}
	m.offset += recSize

	hrec := encodeHint(ts, uint32(len(key)), uint32(len(value)), uint32(recSize), pos, key)
	if _, err := m.hint.Write(hrec); err != nil {
		return Entry{}, err
	}

	return Entry{
		FileID:    m.curID,
		ValuePos:  pos,
		ValueSize: uint32(len(value)),
		RecSize:   uint32(recSize),
		Timestamp: ts,
	}, nil
}

func (m *mergeWriter) finish() error { return m.closeCur() }

// abort closes the output files after an error. Leftover files are harmless:
// recovery prefers newer records and a later merge removes them.
func (m *mergeWriter) abort() {
	if m.data != nil {
		_ = m.data.Close()
		_ = m.hint.Close()
		m.data, m.hint = nil, nil
	}
}
