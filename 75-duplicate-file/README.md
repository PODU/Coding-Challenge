# dupfind — Duplicate File Finder (Coding Challenge #75)

A fast, dependency-free command-line tool that recursively scans directories and
reports files with **identical content** (true duplicates, not just same name).
It can optionally remove duplicates safely, keeping exactly one copy per group.

Like `fdupes`/`rdfind`, it avoids reading every file in full. A progressive
filtering pipeline eliminates non-duplicates as cheaply as possible at each
stage, so expensive work only runs on files that are genuinely likely duplicates:

```
size bucket  ->  partial hash (first 4 KiB)  ->  full hash (MD5)  ->  byte-by-byte verify
```

Single-file buckets are dropped at every stage, so unique files are never fully
hashed. A final byte comparison guarantees that any reported duplicate is truly
identical even in the (astronomically unlikely) event of a hash collision.

## Build

```
go build -o dupfind .
```

Requires Go 1.24+. No third-party dependencies — the result is a single static
binary that runs on Windows, macOS, and Linux.

## Usage

```
dupfind [flags] PATH [PATH...]
```

### Flags

| Flag | Description |
|------|-------------|
| `--min-size BYTES` | Ignore files smaller than BYTES. Accepts suffixes `k`, `M`, `G` (e.g. `4k`, `1M`). Default `1` (skips empty files; pass `0` to include them). |
| `--algo NAME` | Hash algorithm: `md5` (default) or `sha256`. |
| `--format NAME` | Output format: `text` (default) or `json`. |
| `--follow-symlinks` | Follow symbolic links during the walk (cycle-safe). Default off. |
| `--delete` | Delete duplicates, keeping one per group (opt-in). |
| `--dry-run` | With `--delete`, print what would be deleted without touching disk. |
| `--keep STRATEGY` | Which file to keep: `shortest-path` (default), `first-found`, or `interactive`. |
| `--log FILE` | Append all delete actions to FILE. |
| `--quiet` | Suppress the per-group report. |
| `-h, --help` | Usage. |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success, no duplicates found. |
| `1` | Success, duplicates found (fdupes-compatible signal). |
| `2` | Usage error (bad flags / missing path). |
| `3` | Runtime error (e.g. no readable path). |

## Examples

Report duplicates under a tree (human-readable):

```
dupfind ./photos
```

```
Duplicates (size 1048576, 9f86d08188...):
  photos/a/img.jpg
  photos/b/backup/img.jpg

Summary: 1 duplicate group, 1 redundant file, 1.0 MiB reclaimable.
```

Machine-readable output for scripting:

```
dupfind --format json ./photos
```

Preview a cleanup without deleting anything:

```
dupfind --delete --dry-run ./photos
```

Actually delete, keeping the shortest path in each group and logging actions:

```
dupfind --delete --keep shortest-path --log dedupe.log ./photos
```

Only consider files of at least 1 MiB, using SHA-256:

```
dupfind --min-size 1M --algo sha256 ./photos
```

## Behavior & edge cases

- **Empty files** — all zero-byte files are mutual duplicates. They are excluded
  by the default `--min-size 1`; pass `--min-size 0` to report/delete them.
- **Symlinks** — not followed by default (avoids cycles and double counting).
  Symlink entries themselves are never hashed. `--follow-symlinks` opts in and
  tracks visited directories to prevent infinite loops.
- **Hardlinks** — detected via device+inode (Unix). They are grouped, but the
  deleter never removes a hardlink of the keeper (it would free no space) and
  never removes the last link. Detection is disabled on Windows.
- **Permission / unreadable files / files that vanish mid-scan** — captured
  per-file, reported in the summary (and JSON `errors`), never fatal. The rest
  of the scan proceeds.
- **Overlapping roots / a path passed twice** — paths are canonicalized
  (absolute + cleaned) and de-duplicated, so each file is counted once.
- **Large files** — streamed through hashing and verification with fixed
  buffers; never loaded fully into memory.
- **Safety** — deletion is opt-in; the default is read-only reporting. One copy
  per group is always preserved.

## Design

Source layout:

| File | Responsibility |
|------|----------------|
| `main.go` | CLI flag parsing, exit codes, wiring. |
| `walk.go` | Recursive walker (symlink/cycle handling, canonicalization). |
| `pipeline.go` | Size → partial-hash → full-hash bucketing (worker pool). |
| `verify.go` | Byte-by-byte verifier (collision-proof grouping). |
| `report.go` | `text` and `json` output. |
| `delete.go` | Safe, dry-run-aware deleter with keep strategies. |
| `size.go` | `--min-size` suffix parsing. |
| `types.go` | Core data types, `Options`, `Stats`, hasher factory. |
| `filemeta_unix.go` / `filemeta_windows.go` | Platform device/inode lookup. |

## Testing

```
go test ./...          # hermetic tests (build their fixtures under t.TempDir)
go vet ./...
```

Tests cover: duplicate detection on a known tree, the size→partial→full stage
counters, empty-file handling, `--min-size` filtering, byte-verify splitting on
content, dry-run vs. real delete (keeps exactly one, content intact), JSON
schema, exit codes, and size parsing. Permission, symlink, and hardlink tests
are Unix-guarded (skipped on Windows).

> Note: `go test -race` requires cgo; on platforms/toolchains without a working
> 64-bit C compiler the race detector is unavailable, but the standard test run
> is fully functional.
