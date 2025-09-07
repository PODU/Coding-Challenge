# ccbitcask — a Bitcask key-value store

A production-minded implementation of [Bitcask](https://riak.com/assets/bitcask-intro.pdf),
the log-structured hash-table storage engine, in pure Go with **zero third-party
dependencies**. Built for John Crickett's
[Coding Challenge #97](https://codingchallenges.substack.com/p/coding-challenge-97-bitcask).

## What it is

- **All writes are appends** to a single *active* data file. Updates and deletes
  never mutate existing bytes; they append a new record. Every write is one
  sequential disk write, and a torn write at the tail is simply discarded on
  recovery.
- **A complete index lives in RAM** — the *keydir* maps every key to the location
  (file id, offset, size, timestamp) of its latest value. Every `Get` is **one
  seek + one read**, regardless of dataset size, and the data may be far larger
  than RAM.
- **Stale data is reclaimed by merge/compaction**, which rewrites immutable files
  keeping only live values and emits *hint files* so restart can rebuild the
  keydir without reading any values.

## Layout

```
000001.cask   # data file (file id 1); records: crc32 | ts | flags | ksz | vsz | key | value
000002.cask   # ...
000002.hint   # optional hint sidecar accelerating recovery of 000002.cask
000003.cask   # ACTIVE data file (appends go here) — highest id
.lock         # advisory single-writer lock for the directory
```

- File ids are monotonic and zero-padded; higher id = newer. Exactly one active
  file at a time; all others are immutable.
- **Record format** (little-endian, 21-byte header): `crc32(4)` over everything
  after it, `timestamp(8)`, `flags(1)` (bit0 = tombstone), `key_size(4)`,
  `value_size(4)`, then `key` and `value` bytes.
- **Hint record**: `timestamp(8)`, `key_size(4)`, `value_size(4)`,
  `record_size(4)`, `value_pos(8)`, `key`.

## Library API

```go
import bitcask "ccbitcask"

db, err := bitcask.Open(bitcask.Options{
    Dir:          "./mydb",
    MaxFileSize:  64 << 20, // rotate active file at 64 MiB (default 1 GiB)
    SyncOnPut:    false,    // fsync after every write (durable, slower)
    VerifyOnRead: false,    // recompute CRC on every Get (defense in depth)
    ReadOnly:     false,    // open without taking the write lock
})
defer db.Close()

db.Put([]byte("key"), []byte("value"))
v, err := db.Get([]byte("key"))    // ErrKeyNotFound if absent/deleted
db.Delete([]byte("key"))           // appends a tombstone
db.Has([]byte("key"))
db.Keys()                          // snapshot of live keys
db.Fold(func(k, v []byte) error { ... })
db.Merge()                         // compaction + hint generation
db.Sync()                          // fsync the active file
```

Sentinel errors: `ErrKeyNotFound`, `ErrEmptyKey`, `ErrChecksum`,
`ErrDatabaseLocked`, `ErrReadOnly`, `ErrClosed`.

### Concurrency

Many readers run in parallel (`ReadAt` is concurrency-safe and the keydir is
guarded by a `sync.RWMutex`); writes are serialized through a single writer;
`Merge` runs in the background without blocking reads, and never touches the
active file.

## CLI

```
go build ./cmd/ccbitcask

ccbitcask -db <dir> set    <key> <value>   # Put;  prints OK
ccbitcask -db <dir> get    <key>           # prints value, or "(nil)" if missing/deleted
ccbitcask -db <dir> delete <key>           # append tombstone; prints OK
ccbitcask -db <dir> list                   # one live key per line
ccbitcask -db <dir> merge                  # run compaction
```

`-db` defaults to `./database`. Flags: `-sync` (fsync per write),
`-maxfile <bytes>`. Exit codes: `0` success; `2` key-not-found on `get` (still
prints `(nil)`); `1` other errors.

```
$ ccbitcask -db ./database set Key Value
OK
$ ccbitcask -db ./database get Key
Value
```

## Crash recovery

`Open` rebuilds the keydir deterministically:

1. Acquire the directory lock (unless `ReadOnly`).
2. Enumerate `*.cask` files, oldest → newest.
3. For each immutable file, load its hint if valid (no value reads), else scan
   the data file record by record. The active file is always scanned so its tail
   offset is known.
4. **Newest wins** by record timestamp (with file/scan order as tie-break). A
   tombstone removes the key; a later put re-adds it.
5. A torn or CRC-failing tail stops the scan at the last good record; the active
   file is truncated there so future appends are clean. A corrupt value is never
   served.

## Merge / compaction

`Merge` iterates the live keydir, copies each live value (with its original
timestamp) into fresh output files plus paired hint files, fsyncs them, then
atomically repoints the keydir and unlinks the old immutable files. Because it
iterates the keydir, only the latest value per key survives and tombstoned keys
vanish. Each repoint is conditional, so a `Put` racing with a merge is never
lost. Old files are removed only after their replacements are durable, so a crash
mid-merge leaves the authoritative originals intact.

## Tests

```
go test ./...            # hermetic; each test uses its own temp dir
go test ./... -race      # data-race check (needs a 64-bit C toolchain)
```

The suite covers: put/get/delete round-trip, persistence across reopen,
tombstone-across-reopen, re-put after delete, overwrite/newest-wins, CRC
corruption detection (on `Get` and recovery), torn-tail recovery, file rotation,
merge (space reclaimed + latest values kept + deleted keys gone + hint files +
hint-driven reopen), concurrent readers with a live writer, `Keys`/`Fold`, lock
exclusivity, large value gets its own file, and empty-key/empty-value handling.

## Notes & caveats

- **Single writer per directory.** Enforced by an advisory `.lock` file
  (`O_CREATE|O_EXCL`). If a writer process is killed (`SIGKILL`/power loss) it
  leaves a stale `.lock`; remove it manually before reopening. A clean `Close`
  always removes it. `ReadOnly` opens do not take the lock.
- **Recovery resolves conflicts by record timestamp** (`time.Now().UnixNano()`),
  which keeps newest-wins correct even after a merge rewrites live values into
  higher-id files. This assumes a roughly monotonic wall clock; a large backward
  clock jump between writes to the same key could mis-order them. Use `SyncOnPut`
  plus a monotonic clock source if that matters for your deployment.
- **Windows file handling.** Active/merge files are opened `O_RDWR` (not
  `O_APPEND`) with position-based writes, because Windows forbids `Truncate` on
  append-mode handles. Cached read handles for a file are closed before the file
  is unlinked during merge (Windows refuses to delete an open file).
- The `-race` detector requires a 64-bit C toolchain (cgo). The local Windows
  toolchain here is 32-bit-only, so race runs are best done in CI/Linux; all
  tests pass without `-race`.
```
go vet ./...   # clean
gofmt -l .     # clean
go test ./...  # ok (coverage ~74%)
```
