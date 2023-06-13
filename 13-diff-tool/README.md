# Coding Challenge #13 — `diff` Tool

A command-line `diff` clone written in Go. It compares two text files and prints
the line edits needed to turn the first into the second, driven by a Longest
Common Subsequence (LCS) engine — the same idea behind Unix `diff` and Git's
line diffing.

## Build & Install

```sh
go build -o diff ./cmd/diff      # build a local binary
go install ./cmd/diff            # install into $GOBIN
```

No third-party dependencies — standard library only.

## Usage

```
diff [--myers] <file1> <file2>
```

Output:

```
< line   present only in file1 (deleted)
> line   present only in file2 (added)
```

Shared lines are not printed. Within each changed region, deletions are listed
before additions (matching GNU `diff`).

### Example

`a.txt`:

```
apple
banana
cherry
```

`b.txt`:

```
apple
BANANA
cherry
date
```

```sh
$ diff a.txt b.txt
< banana
> BANANA
> date
$ echo $?
1
```

### Flags

| Flag | Description |
|------|-------------|
| `--myers` | Force Eugene Myers's O(ND) algorithm instead of auto-selecting. |
| `-h`, `--help` | Print usage and exit 0. |

### Exit codes (mirror GNU `diff`)

| Code | Meaning |
|------|---------|
| `0` | Files identical — no output. |
| `1` | Differences found — diff printed to stdout. |
| `2` | Error (bad arguments, missing/unreadable file, I/O failure). |

## How it works

The engine has two interchangeable paths that produce the same edit script:

1. **Dynamic-programming LCS** (`internal/diff/lcs.go`) — the classic
   `dp[i][j]` table giving an exact LCS in **O(n·m)** time and space. Used for
   modest inputs and reused for character-level LCS by comparing runes.
2. **Myers O(ND)** (`internal/diff/myers.go`) — Eugene Myers's
   *An O(ND) Difference Algorithm and Its Variations* (1986), the algorithm Git
   uses by default. It runs in **O((n+m)·D)** time, where `D` is the size of the
   edit script, so it is near-linear for similar files and avoids the quadratic
   memory of the DP table.

`Diff` walks both files against the recovered common subsequence and emits
`<` / `>` lines (`internal/diff/diff.go`). `DiffAuto` picks the DP path for
small inputs and switches to Myers once the DP table would exceed ~4M cells.
The CLI layer (`cmd/diff/main.go`) handles argument parsing, file I/O, and exit
codes.

### Edge-case handling

- Identical files → no output, exit 0.
- Either or both files empty → all lines of the non-empty file reported.
- CRLF vs LF — trailing `\r` is stripped so Windows and Unix files compare
  cleanly.
- A final line without a trailing newline is still compared.
- Blank lines and duplicate lines are significant and handled correctly.
- Character-level LCS iterates over runes, so multibyte/Unicode input is safe.

## Project layout

```
13-diff-tool/
├─ README.md
├─ go.mod
├─ cmd/diff/
│  ├─ main.go         # CLI: arg parsing, file I/O, exit codes
│  └─ main_test.go    # CLI integration + golden-output tests
└─ internal/diff/
   ├─ lcs.go          # DP LCS over lines and runes
   ├─ diff.go         # `<` / `>` edit-script generation + algorithm selection
   ├─ myers.go        # Myers O(ND) LCS recovery
   ├─ lcs_test.go     # LCS unit tests + Myers/DP equivalence fuzz
   └─ diff_test.go    # line-diff unit tests
```

## Testing

```sh
go test ./...                 # all unit + integration tests
go test -cover ./...          # with coverage
gofmt -l . && go vet ./...    # formatting + static checks
```

The suite covers:

- **LCS** — the five challenge-supplied cases plus extras (identical, disjoint,
  empty, duplicates, Unicode); length assertions plus exact-subsequence checks.
- **Equivalence fuzz** — 2,000 random inputs confirm the DP and Myers paths
  always agree on the optimal LCS length and both yield valid common
  subsequences.
- **Line diff** — added / removed / changed / empty / identical / mixed cases.
- **CLI golden tests** — added/removed/changed/empty/identical files, CRLF vs
  LF, and the error paths (missing file, wrong arg count, `--help`) with their
  exact stdout and exit codes.

## Notes / caveats

- When two files have more than one equally-optimal LCS, the DP and Myers paths
  may report different (but equally valid and equally short) edit scripts. Both
  are correct; this matches how different real diff implementations can disagree
  on presentation.
- The default mode prints only `<` / `>` lines, as the challenge specifies — it
  does not emit GNU-style hunk headers (`1,2c3`) or context lines.
- `go test -race` requires a 64-bit C toolchain; on a system without one, run
  the plain `go test ./...` (the code itself has no concurrency).
