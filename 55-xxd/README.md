# xxd — Coding Challenge #55

A from-scratch reimplementation of the Unix `xxd` hex-dump utility, in Go with
zero third-party dependencies. It produces the classic three-column dump (byte
offset, hex bytes, printable ASCII gutter) and can reverse a dump back into the
original binary.

The bar is **byte-for-byte parity with the real `xxd`** on the supported flag
set, verified by tests that diff our output against system `xxd` when it is
available.

## Build

```sh
go build ./cmd/xxd        # produces ./xxd (or xxd.exe on Windows)
```

Requires Go 1.24+. No external modules.

## Usage

```
xxd [options] [infile]

Make a hexdump of infile (or stdin) to stdout, or reverse a dump back to binary.

Options:
  -l, -len   <n>   dump at most <n> octets
  -s, -seek  <n>   start dump at byte offset <n> (decimal or 0x-hex)
  -c, -cols  <n>   <n> bytes per output line (default 16; 30 with -p)
  -g, -group <n>   <n> bytes per hex group (default 2; 4 with -e; 0 = none)
  -e               little-endian dump (reverse bytes within each group)
  -p, -ps          plain/postscript dump: continuous hex, no offset/ASCII
  -r               reverse: read a hex dump, write binary to stdout
  -h, -help        show help
  -v, -version     show version
```

Both short and long flag spellings work, and values may be given as `-c 8` or
`-c=8`. Flags and a positional `infile` may appear in any order; `-` means stdin.

### Examples

```sh
$ xxd file1.txt
00000000: 4669 6c65 2031 2063 6f6e 7465 6e74 732e  File 1 contents.
00000010: 0a                                       .

$ xxd -e file1.txt          # little-endian, default group size 4
00000000: 656c6946 63203120 65746e6f 2e73746e  File 1 contents.
00000010:       0a                             .

$ xxd -p file1.txt          # plain / postscript
46696c65203120636f6e74656e74732e0a

$ xxd -c 8 -g 4 file1.txt
00000000: 46696c65 20312063  File 1 c
00000008: 6f6e7465 6e74732e  ontents.
00000010: 0a                 .

# Reverse round-trips
$ xxd file1.txt    | xxd -r        # default dump -> binary
$ xxd -p file1.txt | xxd -r -p     # plain dump  -> binary
```

## How it works

The core logic lives in `internal/xxd` and operates purely on `io.Reader` /
`io.Writer` plus a `Config`, so it is unit-testable without spawning a process.
`cmd/xxd` is a thin CLI shell (flag parsing, input resolution, exit codes).

```
55-xxd/
  go.mod
  cmd/xxd/main.go          CLI: flag parsing, wiring, exit codes
  internal/xxd/
    config.go              Config struct + validation + defaults
    format.go              offset / hex-field / ASCII-gutter builders
    dump.go                Dump(r, w, cfg) — streaming forward formatter
    reverse.go             Reverse(r, w, cfg) — reverse parser
    *_test.go              golden, round-trip, flag, edge-case, real-xxd tests
```

### Forward dump

- Input is streamed through a `bufio.Reader` in `cols`-sized chunks; nothing is
  buffered whole, so arbitrarily large files are fine.
- `-s` seeks via `io.Seeker` on real files and falls back to discarding bytes on
  non-seekable streams (pipes/stdin); `-l` caps the octet count via
  `io.LimitReader`.
- The hex field pads missing trailing byte slots with spaces up to `cols`, so a
  partial last line keeps the ASCII gutter aligned. In `-e` mode the bytes within
  each group are emitted high-slot-first (little-endian), so missing bytes become
  leading spaces.
- The offset column is lowercase hex, zero-padded to at least 8 digits, widening
  automatically for offsets beyond `0xFFFFFFFF`.

### Reverse (`-r`)

- Default mode parses each line's offset (before the first `:`) to position the
  bytes, then decodes the hex column up to the two-space ASCII-gutter separator —
  group separators are single spaces, so the first double-space reliably marks
  the gutter. The gutter and any trailing characters are ignored, and CRLF line
  endings are tolerated.
- Offsets that skip ahead are zero-filled, so seeked / sparse dumps round-trip.
- `-r -p` concatenates every hex digit in the input and decodes one continuous
  stream.

## Testing

```sh
go vet ./...
go test ./...
```

The test suite includes:

- **Golden dumps** — hand-verified, byte-for-byte expected output for the default
  layout and each flag (`-p`, `-e`, `-e -g 2`, `-c 8`, `-g 1`, `-g 0`, `-l`,
  `-s`, `-s -l`, `-c -g` combos, `-c 1`, the 0x00–0xff file).
- **Differential test against real `xxd`** — when `xxd` is on `PATH`, our forward
  output is diffed against it across the input × flag matrix. Skipped (with the
  golden strings as the oracle) when `xxd` is absent, e.g. on bare Windows.
- **Reverse round-trips** — `Dump` then `Reverse` reproduces the original bytes
  for every input, including non-default `-c`/`-g` and plain mode.
- **Flag / unit tests** — endian swap, ASCII gutter (printable vs `.`), offset
  widening, partial-line alignment, config validation (bad `-e` group, negative
  `-l`/`-s`, unknown flag).
- **Edge cases** — empty input, `-l 0`, `-s` past EOF, seeked-dump zero-fill,
  CRLF reverse input, gutter text that looks like hex.
- **CLI smoke tests** — exit codes, stdin/file parity, `0x`-hex seek values,
  `-h`/`-v`.

## Caveats & scope

- The `-v` version string is this project's own, not a copy of system `xxd`'s.
- Per the design spec we are stricter than `xxd` about argument validation:
  negative `-l`/`-s`/`-g` and a non-power-of-two `-g` under `-e` are rejected with
  a clear error rather than silently ignored.
- Argument-error exits use code `2`; runtime/validation errors use `1`; success
  is `0`. (Real `xxd` uses other codes in some cases; the values are not part of
  the parity bar.)
- Out of scope for v1: relative seeks (`-s +N`/`-N`),
  `-o` offset add, `-u` uppercase, `-b` bit dump, `-i` C-include, `-R` color,
  `-a` autoskip.
```
