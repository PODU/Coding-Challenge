package bitcask

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// openTemp opens a fresh database in a hermetic temp dir with the given options
// (Dir is filled in automatically).
func openTemp(t *testing.T, opts Options) *Bitcask {
	t.Helper()
	opts.Dir = t.TempDir()
	b, err := Open(opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func mustPut(t *testing.T, b *Bitcask, k, v string) {
	t.Helper()
	if err := b.Put([]byte(k), []byte(v)); err != nil {
		t.Fatalf("Put(%q): %v", k, err)
	}
}

func mustGet(t *testing.T, b *Bitcask, k, want string) {
	t.Helper()
	got, err := b.Get([]byte(k))
	if err != nil {
		t.Fatalf("Get(%q): %v", k, err)
	}
	if string(got) != want {
		t.Fatalf("Get(%q) = %q, want %q", k, got, want)
	}
}

func mustMissing(t *testing.T, b *Bitcask, k string) {
	t.Helper()
	_, err := b.Get([]byte(k))
	if err != ErrKeyNotFound {
		t.Fatalf("Get(%q) err = %v, want ErrKeyNotFound", k, err)
	}
}

// Test 1: put/get/delete round-trip.
func TestPutGetDeleteRoundTrip(t *testing.T) {
	b := openTemp(t, Options{})
	const n = 200
	for i := 0; i < n; i++ {
		mustPut(t, b, fmt.Sprintf("key-%d", i), fmt.Sprintf("val-%d", i))
	}
	for i := 0; i < n; i++ {
		mustGet(t, b, fmt.Sprintf("key-%d", i), fmt.Sprintf("val-%d", i))
	}
	// Delete the even subset.
	for i := 0; i < n; i += 2 {
		if err := b.Delete([]byte(fmt.Sprintf("key-%d", i))); err != nil {
			t.Fatalf("Delete: %v", err)
		}
	}
	for i := 0; i < n; i++ {
		k := fmt.Sprintf("key-%d", i)
		if i%2 == 0 {
			mustMissing(t, b, k)
		} else {
			mustGet(t, b, k, fmt.Sprintf("val-%d", i))
		}
	}
}

// Test 2: persistence across reopen (keydir rebuilt from disk).
func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	const n = 150
	for i := 0; i < n; i++ {
		mustPut(t, b, fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	b2, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer b2.Close()
	for i := 0; i < n; i++ {
		mustGet(t, b2, fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}
}

// Test 3 & 4: tombstone hides key across reopen; re-put after delete wins.
func TestTombstoneAndRePutAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	mustPut(t, b, "a", "1")
	mustPut(t, b, "b", "2")
	if err := b.Delete([]byte("a")); err != nil {
		t.Fatal(err)
	}
	// re-put b after delete to confirm newest wins
	if err := b.Delete([]byte("b")); err != nil {
		t.Fatal(err)
	}
	mustPut(t, b, "b", "2-new")
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	b2, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	mustMissing(t, b2, "a")      // tombstone honored on recovery
	mustGet(t, b2, "b", "2-new") // re-put after delete wins
}

// Test 5: overwrite / newest-wins, including after reopen.
func TestOverwriteNewestWins(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		mustPut(t, b, "k", fmt.Sprintf("v%d", i))
	}
	mustGet(t, b, "k", "v49")
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	b2, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	mustGet(t, b2, "k", "v49")
}

// Test 6: CRC detects corruption on Get and on recovery.
func TestCRCDetectsCorruption(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir, VerifyOnRead: true})
	if err != nil {
		t.Fatal(err)
	}
	mustPut(t, b, "good", "before")
	mustPut(t, b, "victim", "0123456789abcdef")
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	// Flip a byte inside the victim's value region of the data file.
	path := filepath.Join(dir, dataFileName(1))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a byte near the end (inside the second record's value).
	data[len(data)-3] ^= 0xFF
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Get with VerifyOnRead should surface ErrChecksum.
	b2, err := Open(Options{Dir: dir, VerifyOnRead: true})
	if err != nil {
		t.Fatalf("Open after corruption: %v", err)
	}
	defer b2.Close()
	// "good" precedes the corruption and must survive.
	mustGet(t, b2, "good", "before")
	// "victim" record is corrupt: recovery stops at it, so it is not in keydir,
	// OR (if recovery kept it) VerifyOnRead reports a checksum error. Either way
	// no corrupt value is served.
	if v, err := b2.Get([]byte("victim")); err == nil {
		t.Fatalf("Get(victim) returned %q, want error", v)
	} else if err != ErrKeyNotFound && err != ErrChecksum {
		t.Fatalf("Get(victim) err = %v, want ErrKeyNotFound or ErrChecksum", err)
	}
}

// Test 6b: a header whose value_size no longer fits the record must not panic
// on Get even with VerifyOnRead off (bit rot the CRC does not catch there).
// The keydir is built from the clean record; the on-disk header is corrupted
// afterwards, so recovery keeps the entry but the re-read header is bad.
func TestGetCorruptSizeNoPanic(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	mustPut(t, b, "k", "hello")

	// Corrupt the value_size field in place after the keydir already holds a
	// valid entry. The single record starts at offset 0, so vsizeOffset is the
	// uint32 to blow up; ReadAt on Get re-reads it from disk.
	path := filepath.Join(dir, dataFileName(b.active.id))
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte{0xFF, 0xFF, 0xFF, 0x7F}, int64(vsizeOffset)); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// VerifyOnRead is off, so the CRC is not consulted — Get must bounds-check
	// the header itself and return an error rather than panicking.
	if _, err := b.Get([]byte("k")); err == nil {
		t.Fatalf("Get(k) on corrupt size returned nil error, want failure")
	}
}

// Test 7: torn tail / crash mid-write recovers to the last good record.
func TestTornTailRecovery(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	mustPut(t, b, "k1", "v1")
	mustPut(t, b, "k2", "v2")
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	// Simulate a partial write: append a few junk bytes (an incomplete record).
	path := filepath.Join(dir, dataFileName(1))
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01}); err != nil {
		t.Fatal(err)
	}
	f.Close()

	b2, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatalf("Open after torn tail: %v", err)
	}
	defer b2.Close()
	mustGet(t, b2, "k1", "v1")
	mustGet(t, b2, "k2", "v2")
	// Database must be writable after recovery (tail truncated).
	mustPut(t, b2, "k3", "v3")
	mustGet(t, b2, "k3", "v3")
}

// Test 8: file rotation at a tiny size limit.
func TestFileRotation(t *testing.T) {
	dir := t.TempDir()
	// 200-byte cap forces frequent rotation.
	b, err := Open(Options{Dir: dir, MaxFileSize: 200})
	if err != nil {
		t.Fatal(err)
	}
	const n = 100
	for i := 0; i < n; i++ {
		mustPut(t, b, fmt.Sprintf("key%03d", i), fmt.Sprintf("value-%03d", i))
	}
	ids, err := listDataFileIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) < 2 {
		t.Fatalf("expected multiple data files, got %d", len(ids))
	}
	// Exactly one active file (the highest id).
	if b.active.id != ids[len(ids)-1] {
		t.Fatalf("active id %d, want highest %d", b.active.id, ids[len(ids)-1])
	}
	for i := 0; i < n; i++ {
		mustGet(t, b, fmt.Sprintf("key%03d", i), fmt.Sprintf("value-%03d", i))
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	// All keys still readable across files after reopen.
	b2, err := Open(Options{Dir: dir, MaxFileSize: 200})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	for i := 0; i < n; i++ {
		mustGet(t, b2, fmt.Sprintf("key%03d", i), fmt.Sprintf("value-%03d", i))
	}
}

func dirSize(t *testing.T, dir string) int64 {
	t.Helper()
	var total int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		total += info.Size()
	}
	return total
}

// Test 9 & 10: merge reclaims space, keeps latest values, drops deleted keys,
// emits hint files, and reopen consumes those hints.
func TestMergeReclaimsSpaceAndHints(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir, MaxFileSize: 1024})
	if err != nil {
		t.Fatal(err)
	}
	const keys = 30
	// Bloat the files by overwriting each key many times.
	for round := 0; round < 40; round++ {
		for k := 0; k < keys; k++ {
			mustPut(t, b, fmt.Sprintf("k%02d", k), fmt.Sprintf("round-%02d-val-%02d", round, k))
		}
	}
	// Delete a subset.
	for k := 0; k < keys; k += 3 {
		if err := b.Delete([]byte(fmt.Sprintf("k%02d", k))); err != nil {
			t.Fatal(err)
		}
	}
	// Force a fresh active file so prior writes become immutable and mergeable.
	mustPut(t, b, "sentinel", "x")
	if err := b.Sync(); err != nil {
		t.Fatal(err)
	}

	before := dirSize(t, dir)
	if err := b.Merge(); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	after := dirSize(t, dir)
	if after >= before {
		t.Fatalf("merge did not shrink dir: before=%d after=%d", before, after)
	}

	// Live keys keep latest value; deleted keys gone.
	for k := 0; k < keys; k++ {
		key := fmt.Sprintf("k%02d", k)
		if k%3 == 0 {
			mustMissing(t, b, key)
		} else {
			mustGet(t, b, key, fmt.Sprintf("round-39-val-%02d", k))
		}
	}

	// Hint files must exist for merged outputs.
	hints, _ := filepath.Glob(filepath.Join(dir, "*"+hintExt))
	if len(hints) == 0 {
		t.Fatal("expected hint files after merge")
	}

	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen: recovery must consume hints and reproduce the same live set.
	b2, err := Open(Options{Dir: dir, MaxFileSize: 1024})
	if err != nil {
		t.Fatalf("reopen after merge: %v", err)
	}
	defer b2.Close()
	for k := 0; k < keys; k++ {
		key := fmt.Sprintf("k%02d", k)
		if k%3 == 0 {
			mustMissing(t, b2, key)
		} else {
			mustGet(t, b2, key, fmt.Sprintf("round-39-val-%02d", k))
		}
	}
	mustGet(t, b2, "sentinel", "x")
}

// Regression: after a merge, the compacted output files carry higher ids than
// the active file and each has a paired hint. Recovery must not reopen such a
// merged (hint-paired) file as the active append target; doing so would write
// fresh records into a compacted file behind a stale hint. The active file must
// be the highest-id file with no hint sidecar, and post-reopen writes must
// survive a further reopen.
func TestMergeThenReopenSelectsHintlessActive(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir, MaxFileSize: 1024})
	if err != nil {
		t.Fatal(err)
	}
	const keys = 30
	for round := 0; round < 40; round++ {
		for k := 0; k < keys; k++ {
			mustPut(t, b, fmt.Sprintf("k%02d", k), fmt.Sprintf("round-%02d-val-%02d", round, k))
		}
	}
	mustPut(t, b, "sentinel", "x") // rotate prior writes into an immutable file
	if err := b.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := b.Merge(); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	b2, err := Open(Options{Dir: dir, MaxFileSize: 1024})
	if err != nil {
		t.Fatalf("reopen after merge: %v", err)
	}
	// The reopened active file must not have a paired hint (i.e. must not be a
	// merged output file).
	activeHint := filepath.Join(dir, hintFileName(b2.active.id))
	if _, err := os.Stat(activeHint); err == nil {
		t.Fatalf("active file %d has a paired hint file: a merged file was reopened as active", b2.active.id)
	}

	// Overwrite every key after reopen, then reopen once more and verify.
	for k := 0; k < keys; k++ {
		mustPut(t, b2, fmt.Sprintf("k%02d", k), fmt.Sprintf("new-val-%02d", k))
	}
	if err := b2.Close(); err != nil {
		t.Fatal(err)
	}

	b3, err := Open(Options{Dir: dir, MaxFileSize: 1024})
	if err != nil {
		t.Fatalf("second reopen: %v", err)
	}
	defer b3.Close()
	for k := 0; k < keys; k++ {
		mustGet(t, b3, fmt.Sprintf("k%02d", k), fmt.Sprintf("new-val-%02d", k))
	}
	mustGet(t, b3, "sentinel", "x")
}

// Test 11: concurrent reads while a writer runs (use -race).
func TestConcurrentReads(t *testing.T) {
	b := openTemp(t, Options{VerifyOnRead: true})
	const keys = 64
	for i := 0; i < keys; i++ {
		mustPut(t, b, fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Single writer continuously overwriting.
	wg.Add(1)
	go func() {
		defer wg.Done()
		round := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			for i := 0; i < keys; i++ {
				_ = b.Put([]byte(fmt.Sprintf("k%d", i)), []byte(fmt.Sprintf("v%d-%d", i, round)))
			}
			round++
		}
	}()

	// Many concurrent readers; never observe a corrupt/partial value.
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				i := j % keys
				v, err := b.Get([]byte(fmt.Sprintf("k%d", i)))
				if err == ErrKeyNotFound {
					continue
				}
				if err != nil {
					t.Errorf("Get: %v", err)
					return
				}
				if !bytes.HasPrefix(v, []byte(fmt.Sprintf("v%d", i))) {
					t.Errorf("Get(k%d) = %q, unexpected", i, v)
					return
				}
			}
		}()
	}
	// Let it run a bit then stop the writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(stop)
	}()
	wg.Wait()
}

// Test 12: Keys and Fold return exactly the live set.
func TestKeysAndFold(t *testing.T) {
	b := openTemp(t, Options{})
	want := map[string]string{}
	for i := 0; i < 20; i++ {
		k := fmt.Sprintf("k%d", i)
		v := fmt.Sprintf("v%d", i)
		mustPut(t, b, k, v)
		want[k] = v
	}
	// Delete some.
	for i := 0; i < 20; i += 4 {
		k := fmt.Sprintf("k%d", i)
		if err := b.Delete([]byte(k)); err != nil {
			t.Fatal(err)
		}
		delete(want, k)
	}

	keys := b.Keys()
	if len(keys) != len(want) {
		t.Fatalf("Keys len = %d, want %d", len(keys), len(want))
	}
	for _, k := range keys {
		if _, ok := want[string(k)]; !ok {
			t.Fatalf("Keys returned unexpected key %q", k)
		}
	}

	got := map[string]string{}
	if err := b.Fold(func(k, v []byte) error {
		got[string(k)] = string(v)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("Fold visited %d, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("Fold[%q] = %q, want %q", k, got[k], v)
		}
	}
}

// Test 13: lock exclusivity — a second writable Open fails.
func TestLockExclusivity(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	if _, err := Open(Options{Dir: dir}); err != ErrDatabaseLocked {
		t.Fatalf("second Open err = %v, want ErrDatabaseLocked", err)
	}

	// A read-only Open does not take the lock and should succeed.
	ro, err := Open(Options{Dir: dir, ReadOnly: true})
	if err != nil {
		t.Fatalf("read-only Open: %v", err)
	}
	ro.Close()

	// After closing the writer, a fresh writable Open succeeds.
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b2, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatalf("reopen after close: %v", err)
	}
	b2.Close()
}

// Test 14: a value larger than the rotation unit gets its own file.
func TestLargeValueOwnFile(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir, MaxFileSize: 256})
	if err != nil {
		t.Fatal(err)
	}
	big := bytes.Repeat([]byte("Z"), 4096) // far exceeds MaxFileSize
	mustPut(t, b, "small1", "a")
	if err := b.Put([]byte("huge"), big); err != nil {
		t.Fatal(err)
	}
	mustPut(t, b, "small2", "b")

	got, err := b.Get([]byte("huge"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, big) {
		t.Fatalf("huge value mismatch: got %d bytes", len(got))
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	b2, err := Open(Options{Dir: dir, MaxFileSize: 256})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()
	got2, err := b2.Get([]byte("huge"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got2, big) {
		t.Fatalf("huge value mismatch after reopen")
	}
	mustGet(t, b2, "small1", "a")
	mustGet(t, b2, "small2", "b")
}

// Empty key is rejected; empty value is allowed and distinct from a tombstone.
func TestEmptyKeyAndEmptyValue(t *testing.T) {
	b := openTemp(t, Options{})
	if err := b.Put(nil, []byte("x")); err != ErrEmptyKey {
		t.Fatalf("Put(empty key) err = %v, want ErrEmptyKey", err)
	}
	if err := b.Put([]byte("k"), []byte{}); err != nil {
		t.Fatalf("Put empty value: %v", err)
	}
	v, err := b.Get([]byte("k"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(v) != 0 {
		t.Fatalf("empty value = %q, want empty", v)
	}
	if !b.Has([]byte("k")) {
		t.Fatal("Has(k) = false, want true for empty value")
	}
}

// After Close, mutating and reading operations fail cleanly with ErrClosed
// rather than reopening (and leaking) file handles into a DB that can no longer
// close them. Close itself is idempotent.
func TestUseAfterClose(t *testing.T) {
	dir := t.TempDir()
	b, err := Open(Options{Dir: dir})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	mustPut(t, b, "k", "v")
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := b.Get([]byte("k")); err != ErrClosed {
		t.Fatalf("Get after Close err = %v, want ErrClosed", err)
	}
	if err := b.Put([]byte("k"), []byte("v2")); err != ErrClosed {
		t.Fatalf("Put after Close err = %v, want ErrClosed", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("second Close should be a no-op, got %v", err)
	}
}
