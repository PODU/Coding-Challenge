# Coding Challenge #120 — `md5sum`

A production-grade clone of the Unix `md5sum` utility, written in Go.

The MD5 algorithm is **implemented by hand** (RFC 1321) — the padding, the
64-round compression function, and the four-word state machine are all written
from first principles in [`internal/md5`](internal/md5/md5.go). The standard
library's `crypto/md5` is used **only in tests** as an independent oracle, never
in the shipping code path.

## Build & Run

```sh
go build -o md5sum .      # produces ./md5sum (or md5sum.exe on Windows)
go test ./...             # run the full test suite
go vet ./...              # static checks
```

No third-party dependencies — pure Go standard library, builds on
Windows/Linux/macOS.

## Usage

```
md5sum [OPTION]... [FILE]...

With no FILE, or when FILE is "-", read standard input.

  -c, --check          read checksums from the FILEs and verify them
  -b, --binary         read in binary mode (output "*" before filename)
  -t, --text           read in text mode (default)
      --tag            create a BSD-style (reversed) checksum line
      --quiet          (with -c) don't print OK for each verified file
      --status         (with -c) silent; signal result via exit code only
  -a, --algorithm A    digest algorithm: md5 (default) or sha256
      --help           display this help and exit
      --version        output version information and exit
```

### Examples

```sh
md5sum file.txt                 # e59ff97941044f85df5297e1c302d260  file.txt
md5sum a.txt b.txt              # one line per file
printf abc | md5sum             # 900150983cd24fb0d6963f7d28e17f72  -
md5sum --tag file.txt           # MD5 (file.txt) = e59ff979...
md5sum -b file.txt              # <hex> *file.txt   (binary marker)

md5sum file.txt > sums.md5
md5sum -c sums.md5              # file.txt: OK
md5sum -c --quiet sums.md5      # prints only FAILED lines
md5sum -c --status sums.md5     # prints nothing; result via exit code
```

### Output formats

| Mode      | Line format                          |
|-----------|--------------------------------------|
| default   | `<hex>␣␣<name>`                      |
| `-b`      | `<hex>␣*<name>`                      |
| `--tag`   | `MD5 (<name>) = <hex>`               |

### Exit status

- `0` — success / all checks passed.
- `1` — a digest mismatch, an unreadable file, a malformed checksum line (in
  `-c` mode), or a hashing/IO error in normal mode.
- `2` — usage error (unknown flag, mutually exclusive options, etc.).

## Design

- **Streaming, O(1) memory.** `Digest` implements `hash.Hash`; input is consumed
  block-by-block via `io.CopyBuffer` in 32 KiB chunks, so arbitrarily large
  files never get loaded into RAM.
- **`-c` check mode** accepts both the GNU format (`<hex>␣[␣|*]<name>`) and the
  BSD `--tag` format (`MD5 (<name>) = <hex>`), tolerates trailing `\r` (Windows
  CRLF), compares digests case-insensitively, and reports per-file
  `OK`/`FAILED` plus a coreutils-style stderr summary.
- **Algorithm-agnostic plumbing.** The file/stdin reader, formatter, and check
  machinery work over a small `algorithm` abstraction, so the optional
  `-a sha256` stretch (a second hand-written core in
  [`internal/sha256`](internal/sha256/sha256.go)) reuses everything unchanged —
  only the digest engine and the BSD tag label differ.

## Tests

`go test ./...` covers:

- **RFC 1321 known-answer vectors** (`""`, `"a"`, `"abc"`, `"message digest"`,
  the alphabet, the 62-char alnum string, and 80×`"1234567890"`).
- **Differential vs `crypto/md5`** for every length `0..400` (nailing the
  55/56/57, 63/64/65, 119/120 padding boundaries) plus multi-MB random inputs.
- **Streaming invariance** — one big `Write` vs many 1/7/63/64/65-byte `Write`s.
- **`FuzzMD5`** — random byte slices, ours vs `crypto/md5`.
- **Output formatting** golden strings (default / `-b` / `--tag`, incl. a
  filename with spaces).
- **Check mode** — pass, mismatch (`FAILED`), missing file
  (`FAILED open or read`), malformed line, `--quiet`, `--status`, and
  `--tag`/`-b` round-trips.
- **CLI integration** ([`main_test.go`](main_test.go)) — builds the binary and
  drives it via `os/exec`: multiple files, stdin (no args and explicit `-`),
  and missing-file exit codes.

## Layout

```
120-md5sum/
├── go.mod
├── main.go                    # thin entrypoint → app.Run
├── main_test.go               # os/exec CLI integration tests
├── internal/
│   ├── md5/    md5.go md5_test.go        # hand-written MD5 core (hash.Hash)
│   ├── sha256/ sha256.go sha256_test.go  # hand-written SHA-256 (stretch)
│   └── app/    app.go app_test.go        # flag parsing, formatting, check mode
└── README.md
```
