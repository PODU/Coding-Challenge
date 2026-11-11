# Disk Analyser — Coding Challenge #145

`ccdisk` is a terminal disk space analyser in the spirit of DaisyDisk,
GrandPerspective and WinDirStat. It scans a directory tree in parallel and
draws it as a nested, squarified treemap. It shows what is large, old and
reclaimable, and moves only the items you choose to the Trash or Recycle Bin,
after a review step.

Source: https://codingchallenges.substack.com/p/coding-challenge-145-disk-analyser

## Build & test

```sh
go build -o ccdisk .     # ccdisk.exe on Windows
go vet ./...
go test ./...            # builds the challenge's test tree itself, ~100 MB in a temp dir
CCDISK_TEST_RECYCLE=1 go test ./internal/cleanup   # Windows: also exercise the real Recycle Bin
```

Requires Go 1.24+. Standard library only, no third-party dependencies. Runs on
Linux, macOS, the BSDs and Windows 10+ (64-bit for the Recycle Bin).

## Usage

```
ccdisk [flags] [DIR]          interactive treemap of DIR (default .)
ccdisk --report [flags] DIR   the same analysis as plain text

  -apparent       show apparent size instead of disk usage
  -x              stay on one filesystem
  -workers N      directories read at once (default 4 per CPU)
  -depth N        report: tree depth (default 2)
  -top N          report: number of largest files (default 10)
  -min-size SIZE  report: e.g. 10M, 1G
  -min-age AGE    report: e.g. 30d, 6mo, 1y
```

### Keys

| Key | Action |
|---|---|
| ←↑→↓ | move to the neighbouring rectangle (climbs out of nested folders) |
| `]` / `[` | select inside / select the parent |
| ⏎, double-click | open the selected folder so it fills the treemap |
| ⌫ or `u` | back up one level |
| `t`, Home, click a breadcrumb | jump to that level (the top in one action) |
| click | select |
| `a` | disk usage ⇄ apparent size (always labelled in the header) |
| `g` | colour by file type ⇄ by age (< 1 month, < 1 year, > 1 year) |
| `i` / `l` / `w` | Info panel / Largest files / Warnings |
| `m` / `o` | largest-files filter: minimum size / minimum age |
| `1`–`9` | jump to the Nth largest child shown in Info |
| Tab | move keyboard focus to the panel (↑↓ ⏎) and back |
| space or `s` | stage / unstage for removal (no disk change) |
| `c` | review the staged items: `d` drops one, `y` then `y` confirms, Esc cancels |
| `r`, F5 | rescan the open folder only |
| `q` | quit |

In the treemap, a ♻ marks a cache or build-output folder, ✗ and hatching mark
staged items, and the selection has a yellow double frame or a white fill.
Children too small to read are merged into one "N smaller" region.

## Challenge steps

| Step | Feature | Status |
|---|---|---|
| 1 | Read-only scan into memory, warnings for unreadable directories | ✅ item count = `find`; hashes unchanged |
| 2 | Disk usage and apparent size, totals, file counts, mtimes, % of parent and of scan | ✅ sparse 1 GB apparent / 0 used |
| 3 | Hard links counted once, links not followed, parallel scan with live progress | ✅ media = 50 MB, matching `du -sk` |
| 4 | Squarified treemap, visible nesting, "smaller items" grouping | ✅ video:photo1 ≈ 5:1 |
| 5 | Selection, open/back/breadcrumbs, relayout on resize | ✅ |
| 6 | Inspector, "40 MB, 80% of testroot/media", jump to the largest children | ✅ |
| 7 | Age view and legend, directory age = newest descendant | ✅ |
| 8 | Largest files by disk usage, size and age filters, scoped to the open folder | ✅ |
| 9 | Cache and build-output detection, reclaimable total | ✅ |
| 10 | Staging only, totals with no double counting, free now → predicted | ✅ 15 → 36 MB, hard link once |
| 11 | Review screen, protected paths, re-check before each removal | ✅ vanished and kind-changed items skipped |
| 12 | Trash or Recycle Bin only, failures reported, rescan, moved vs. actual free-space change | ✅ |
| Extra | keyboard-only mode, rescan the current folder, text report | ✅ |

## Verification on this machine (Windows 11)

- `go test ./...`: 64 tests across scan, treemap, analysis, cleanup, term, tui
  and the CLI. They run against the challenge's own test tree, including a real
  hard link, a real NTFS sparse file and a junction.
- I built the challenge's tree with its own shell commands. On Windows I used
  `fsutil sparse` and an `icacls` deny ACL in place of `chmod 000`. On that
  tree, `--report` agrees with `du -sk testroot/media` (51200 KB),
  `du -k sparse.img` (0) and `find testroot | wc -l` (28), and warns about
  `locked`.
- The real Recycle Bin test moves a file into the bin, and correctly reports a
  failure for a file that another process holds open.
- I ran the UI in Windows Terminal, drove it with key presses and checked it
  with screenshots.
- Linux, macOS and FreeBSD builds pass `go vet` but were not run. The
  freedesktop Trash tests (`trash_xdg_test.go`) run only on Linux.
