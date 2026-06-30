# `du` — disk usage (Coding Challenge #124)

A production-quality clone of the Unix `du` utility, written in Go. It walks a
directory tree, accumulates file sizes **bottom-up**, and prints a
per-directory summary deepest-first.

## Build & run

```sh
go build -o du .         # produces ./du (or du.exe on Windows)
./du                     # summarize the current directory
./du -sh /some/path      # human-readable grand total for a path
./du -ah --max-depth=2 . # all files, human units, two levels deep
```

## Usage

```
du [OPTIONS] [PATH...]      (default PATH is ".")

Units (last wins):
  -h, --human-readable   K/M/G/T suffixes (1024-based)
  -k                     1 KiB blocks
  -m                     1 MiB blocks
      --block-size=SIZE  custom block size (e.g. 1K, 1M)
      --apparent-size    logical size instead of allocated blocks
  -b, --bytes            apparent size in bytes (implies --apparent-size)

Output selection:
  -a, --all              include individual files
  -s, --summarize        only a total per argument (implies depth 0)
  -d, --max-depth=N      print entries up to N levels deep
  -c, --total            append a grand 'total' line
  -t, --threshold=SIZE   only entries >= SIZE (or <= |SIZE| if negative)

Traversal:
  -L, --dereference      follow symlinks, count target contents
  -x, --one-file-system  stay on one filesystem
      --exclude=PATTERN  skip entries matching glob (repeatable)
      --exclude-from=F   read exclude globs from a file (repeatable)

Misc:
      --inodes           count inodes instead of bytes
      --help             show help
```

Output line format is TSV: `size<TAB>path`. The grand total (`-c`) is
`size<TAB>total`.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | some entries could not be read (permission, missing); a partial result is still printed |
| 2 | usage error (bad flag combination, unparsable size) |

## How sizes are computed

Sizes are accumulated in **bytes**; the unit conversion happens only at print
time. Each directory's reported size is the sum of every file beneath it plus,
in allocated mode, the directory inode's own blocks. Totals roll up from leaves
to root (post-order traversal), so a buffer of entries is naturally
deepest-first.

- **Allocated (default)** — bytes actually allocated on disk. A 1-byte file
  occupies a whole block, so it is charged a full block.
- **Apparent (`--apparent-size` / `-b`)** — the logical byte length
  (`info.Size()`). In apparent mode a directory's own size is counted as **0**,
  because a directory's logical "size" is filesystem-dependent and not
  meaningful for a roll-up; this keeps apparent totals deterministic across
  platforms (and is what the test suite asserts).

Hardlinks: a physical file with link count > 1 is charged its full size the
**first** time its `(device, inode)` is seen and **zero** thereafter, so it is
counted exactly once across the whole invocation (all roots).

Symlinks: by default the link itself is counted (its small `Lstat` size) and
not traversed. With `-L` the target is `Stat`-ed and traversed; a resolved-path
visited set breaks cycles (a re-encountered directory counts 0).

## Windows vs Unix block-size behavior (important)

The platform-specific metadata lookup is isolated behind a build-tagged seam:
[`internal/platform/stat_unix.go`](./internal/platform/stat_unix.go) and
[`internal/platform/stat_windows.go`](./internal/platform/stat_windows.go),
both implementing a common `Meta` type (`Size`, `Allocated`, `Dev`, `Ino`,
`Nlink`). The walker and sizer are pure and platform-independent.

| | Unix | Windows |
|---|------|---------|
| Allocated size | `512 * st_blocks` — **true** on-disk allocation | logical size rounded up to a **4096-byte cluster** (approximation) |
| Sparse files | apparent ≫ allocated (reflected correctly) | not reflected (no block metadata) |
| Hardlink dedup | by `(st_dev, st_ino)` | **disabled** — no inode identity in the standard library `FileInfo.Sys()` |

On Unix, `syscall.Stat_t` exposes `Blocks`, `Dev`, `Ino`, and `Nlink`, giving
exact allocation and reliable dedup. On Windows, the standard library's
`FileInfo.Sys()` is `*syscall.Win32FileAttributeData`, which carries **no**
allocated-block count and **no** inode identity, so true allocation and dedup
would require `GetFileInformationByHandle` via `golang.org/x/sys/windows`. To
keep the dependency surface at zero, this implementation approximates allocation
by cluster rounding and reports `Nlink == 1` (dedup off). These differences are
expected and the test suite accounts for them: block-based and hardlink
assertions are gated behind `//go:build !windows`, while the cross-platform
tests use **apparent** sizing for deterministic, host-independent expectations.

## Tests

```sh
go test ./...
```

Every test is hermetic: it builds its own tree under `t.TempDir()` with files of
known sizes. Coverage includes bottom-up accumulation, `-s`, `-d`, `-a`, `-h`
formatting, `--exclude` globs, `-c` grand total, `-t` threshold,
permission/missing-path handling, and (Unix only) hardlink dedup and a
`chmod 000` permission error.
