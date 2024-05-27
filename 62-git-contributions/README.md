# gitgraph — Git Contributions Graph

A small, dependency-free Go CLI that scans local git repositories, aggregates
commits **per day per author**, and renders a GitHub-style contributions
heatmap with summary statistics (totals, streaks, busiest day).

It is **fully offline** — it never touches the network. History is read by
shelling out to the installed `git` binary, with a best-effort direct reader
(`.git/logs/HEAD`) used as a fallback when `git` is not on `PATH`.

This implements [Coding Challenge #62](https://codingchallenges.substack.com/p/coding-challenge-62-git-contributions).

## Build

```sh
go build -o gitgraph ./cmd/gitgraph     # single static binary, stdlib only
```

## Quick start

```sh
gitgraph                                   # last 30 days, you, cwd, terminal
gitgraph --days 365 ~/code                 # a year across all repos under ~/code
gitgraph --all-authors --leaderboard .     # team heatmap + contributor leaderboard
gitgraph --format svg --out graph.svg --days 365 ~/code
gitgraph --author "ada@" --tz UTC --since 2026-01-01
```

Example terminal output:

```
Contributions: all authors | myrepo | 14 days
    Jun
Sun . #
Mon . #
Tue . #
Wed . .
Thu . .
Fri . .
Sat . .

Less . : + * #  More

Total:    3 commits in 14 days
Active:   3 days
Max/day:  1
Streaks:  current 0, longest 3
Busiest:  2026-06-21 (1)
```

(In a real terminal the cells are coloured green blocks; the ASCII ramp shown
here is used automatically when output is piped or `--no-color` is set.)

## Features

- **Repository discovery** — recursively finds every directory containing a
  `.git` entry (directory *or* file, covering worktrees/submodules). Does not
  descend into a repo by default; `--nested` opts in.
- **Timezone-correct day bucketing** — each commit is converted into the
  viewing timezone (`--tz`, default local) before being assigned a calendar day,
  so a commit at `23:30 -05:00` lands on the right day in `America/New_York`
  vs `UTC`.
- **Statistics** — total commits, active days, max/day, current & longest
  streak, busiest day. Per-contributor leaderboard with `--leaderboard`.
- **Rendering** — ANSI/Unicode terminal heatmap (auto-detects TTY for colour),
  or `--format svg` / `--format html` for a pixel-accurate GitHub-style export.
- **Caching** (`O4`) — per-repo commit cache under the OS cache dir; reruns read
  only `lastSHA..HEAD`. `--no-cache` and `--refresh` control it.
- **Multi-repo aggregation** with cross-repo SHA de-duplication.

## CLI reference

```
gitgraph [flags] [PATH ...]

Range
  --days N          Analyse the previous N days (default 30).
  --since DATE      Explicit start date YYYY-MM-DD (overrides --days).
  --until DATE      Explicit end date YYYY-MM-DD (default today).
  --tz NAME         IANA timezone for day bucketing (default local).
  --first-day D     Week starts on sun|mon (default sun).

Authors
  --author STR      Filter to commits whose name OR email contains STR.
  --me              Filter to each repo's git user.email (default).
  --all-authors     Aggregate every author.
  --include-merges  Count merge commits (default off).

Discovery
  --nested          Descend into nested repositories.
  --all-branches    Include commits from all branches (git log --all).

Output
  --format FMT      terminal | svg | html (default terminal).
  --out FILE        Write svg/html to FILE (default stdout).
  --no-color        Disable ANSI colour; use ASCII intensity ramp.
  --color           Force colour even when piped.
  --stats-only      Print stats only, skip the grid.
  --leaderboard     Show contributor leaderboard (implies --all-authors).

Cache
  --no-cache        Ignore and do not write the cache.
  --refresh         Force a full recompute, then update the cache.

Misc
  --version, --help
```

Exit codes: `0` success; `1` runtime error (bad path, git failure);
`2` usage error (bad flags).

## Architecture

```
cmd/gitgraph        CLI parsing + orchestration
internal/discovery  find .git repos under a root (R1)
internal/gitlog     read commits: exec `git log` | direct .git fallback (R2)
internal/model      shared types: Commit, Date, Calendar, Stats
internal/aggregate  pure: filter, tz-bucket, dedup, stats, streaks (R2/R4/O1/O3)
internal/calendar   build the week×weekday grid + intensity levels (R5)
internal/cache      JSON per-repo incremental cache (O4)
internal/render/terminal   ANSI/ASCII heatmap + stats panel (R5)
internal/render/svghtml    SVG + standalone HTML export (R5)
```

The `aggregate` and `calendar` packages are pure (no I/O), making the core logic
exhaustively unit-testable.

## Tests

```sh
go test ./...                 # full hermetic suite (default: no network)
go test ./... -cover          # with coverage
go test ./... -race           # race detector (needs a 64-bit C toolchain)
go test ./internal/render/... -update   # regenerate renderer golden files
```

- **Unit tests** (offline, synthetic data): day bucketing incl. timezone
  boundaries, author filtering, date-window inclusivity, streak logic, cross-repo
  de-dup, calendar layout/padding/intensity, cache round-trip & incremental
  merge, civil-date arithmetic.
- **Golden tests**: terminal (colour + no-colour + stats-only) and SVG/HTML
  output are byte-compared against committed `testdata/*.golden` files.
- **Integration tests**: build a real temporary git repo with pinned
  `GIT_AUTHOR_DATE` commits and parse it with the exec reader. These
  **`t.Skip` automatically when `git` is not on `PATH`**, so the default suite
  never requires git or a network.

## Notes & caveats

- **Direct (`git`-less) reader is best-effort.** When `git` is absent the tool
  falls back to parsing `.git/logs/HEAD` (the reflog), which covers the common
  linear case but does not reconstruct full multi-branch history the way
  `git log` does. Install `git` for complete results.
- **Cache & window.** The cache stores raw commits covering the window first
  seen; if a later run requests an *earlier* `--since` than is cached, that repo
  is fully re-read automatically. Use `--refresh` to force a clean recompute.
- **`--race`** requires a 64-bit C toolchain (cgo); it is exercised in CI rather
  than necessarily on every dev machine.
