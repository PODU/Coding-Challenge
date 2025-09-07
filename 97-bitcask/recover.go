package bitcask

import (
	"io"
	"os"
	"path/filepath"
)

// recover rebuilds the keydir from the on-disk data/hint files and prepares the
// active file for appends. It runs single-threaded during Open, so it touches
// the keydir without locking.
//
// Conflict resolution is by record timestamp ("newest wins"), with ascending
// file-id/scan order as the tie-break. Using the timestamp rather than relying
// purely on file order keeps recovery correct even after a merge has rewritten
// live values into higher-id files (the merged records keep their original,
// older timestamps).
func (b *Bitcask) recover() error {
	ids, err := listDataFileIDs(b.opts.Dir)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		b.nextID.Store(0)
		if b.opts.ReadOnly {
			return nil // nothing on disk; no writer
		}
		return b.createActive(b.allocFileID())
	}

	highest := ids[len(ids)-1]
	b.nextID.Store(highest)

	// Pick the active file: the highest-id data file that has no paired hint.
	// Merge always emits a hint sidecar and never merges the active file, so a
	// merged (immutable) output must never be reopened as active — otherwise new
	// appends would be written into a compacted file behind a stale hint. The
	// active file, by contrast, never has a hint. If every file is hinted (all
	// data is immutable), there is no active file to reopen and we start a fresh
	// one below.
	activeIdx := -1
	for i := len(ids) - 1; i >= 0; i-- {
		hintPath := filepath.Join(b.opts.Dir, hintFileName(ids[i]))
		if _, err := os.Stat(hintPath); os.IsNotExist(err) {
			activeIdx = i
			break
		}
	}

	latest := make(map[string]int64)
	for i, id := range ids {
		isActive := i == activeIdx
		dataPath := filepath.Join(b.opts.Dir, dataFileName(id))

		if isActive {
			// The active file is never paired with a hint; always scan it so we
			// learn the precise end offset and can truncate any torn tail.
			goodOffset, err := b.scanData(id, dataPath, latest)
			if err != nil {
				return err
			}
			if b.opts.ReadOnly {
				continue
			}
			if err := b.openActiveAt(id, dataPath, goodOffset); err != nil {
				return err
			}
			continue
		}

		// Immutable file: prefer the hint, fall back to a full scan.
		info, err := os.Stat(dataPath)
		if err != nil {
			return err
		}
		hintPath := filepath.Join(b.opts.Dir, hintFileName(id))
		if _, err := os.Stat(hintPath); err == nil {
			if err := b.loadHint(id, hintPath, info.Size(), latest); err == nil {
				continue
			}
		}
		if _, err := b.scanData(id, dataPath, latest); err != nil {
			return err
		}
	}

	// No hintless file existed (every data file is immutable/merged). Open a new
	// active file with a fresh id so writes never land in a compacted file.
	if activeIdx == -1 && !b.opts.ReadOnly {
		return b.createActive(b.allocFileID())
	}
	return nil
}

// createActive creates an empty active file. It is opened O_RDWR rather than
// O_APPEND for the same reason as in openActiveAt.
func (b *Bitcask) createActive(id uint32) error {
	path := filepath.Join(b.opts.Dir, dataFileName(id))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	b.active = &activeFile{id: id, f: f, offset: 0}
	return nil
}

// openActiveAt opens an existing data file as the active file, truncating any
// torn tail past goodOffset and seeking there so future writes are clean
// appends. (O_APPEND is intentionally avoided: on Windows it forbids Truncate.)
func (b *Bitcask) openActiveAt(id uint32, path string, goodOffset int64) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	if goodOffset < info.Size() {
		if err := f.Truncate(goodOffset); err != nil {
			f.Close()
			return err
		}
	}
	if _, err := f.Seek(goodOffset, io.SeekStart); err != nil {
		f.Close()
		return err
	}
	b.active = &activeFile{id: id, f: f, offset: goodOffset}
	return nil
}

// scanData replays a data file record by record, updating the keydir. It
// returns the offset just past the last good record; a torn or CRC-failing tail
// stops the scan there.
func (b *Bitcask) scanData(id uint32, path string, latest map[string]int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	fileSize := info.Size()

	hdr := make([]byte, headerSize)
	var offset int64
	for offset+headerSize <= fileSize {
		if _, err := f.ReadAt(hdr, offset); err != nil {
			break // short header => torn tail
		}
		h := decodeHeader(hdr)
		recSize := h.recordLen()
		// Guard against absurd sizes from a corrupt header before allocating.
		if recSize <= 0 || offset+recSize > fileSize {
			break // torn tail
		}
		rec := make([]byte, recSize)
		if n, err := f.ReadAt(rec, offset); err != nil || int64(n) < recSize {
			break // torn tail
		}
		if !verifyCRC(rec) {
			break // corrupt/torn record: stop at last good offset
		}
		key := string(rec[headerSize : headerSize+int(h.keySize)])
		if h.isTombstone() {
			b.applyTombstone(latest, key, h.timestamp)
		} else {
			b.applyPut(latest, key, h.timestamp, Entry{
				FileID:    id,
				ValuePos:  offset,
				ValueSize: h.valueSize,
				RecSize:   uint32(recSize),
				Timestamp: h.timestamp,
			})
		}
		offset += recSize
	}
	return offset, nil
}

// applyPut installs e for key unless an already-seen record has a strictly
// newer timestamp.
func (b *Bitcask) applyPut(latest map[string]int64, key string, ts int64, e Entry) {
	if prev, ok := latest[key]; ok && ts < prev {
		return
	}
	latest[key] = ts
	b.keydir[key] = e
}

// applyTombstone removes key unless an already-seen record has a strictly newer
// timestamp.
func (b *Bitcask) applyTombstone(latest map[string]int64, key string, ts int64) {
	if prev, ok := latest[key]; ok && ts < prev {
		return
	}
	latest[key] = ts
	delete(b.keydir, key)
}
