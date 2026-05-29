# dd — Coding Challenge #121

A production-ready reimplementation of the classic Unix `dd` ("convert and copy")
utility in Go. It reads data in fixed-size blocks, optionally transforms each
block, and writes it out, driven entirely by `operand=value` arguments.

## Build & test

```sh
go build -o dd .      # produces a single static binary
go vet ./...
go test ./...
```

Requires Go 1.24+. No third-party dependencies.

## Usage

```
dd [operand=value ...]

OPERANDS
  if=FILE        read from FILE (default: stdin)
  of=FILE        write to FILE  (default: stdout)
  bs=N           input AND output block size (overrides ibs/obs)
  ibs=N          input block size                       (default 512)
  obs=N          output block size                      (default 512)
  count=N        copy only N input blocks
  skip=N         skip N input blocks before copying
  seek=N         skip N output blocks before writing
  conv=L[,L...]  ucase,lcase,swab,notrunc,noerror,sync,sparse
  status=LEVEL   none | noxfer | progress
  iflag=L[,L...] fullblock,direct          (platform-dependent)
  oflag=L[,L...] dsync,direct              (platform-dependent)

SIZES accept suffixes: c=1 w=2 b=512 k/K=1024 M=1024^2 G=1024^3 ; "NxM" multiplies

EXIT  0 success ; 1 usage/operand error ; 2 I/O error
```

### Examples

```sh
dd if=in.txt of=OUT.txt conv=ucase            # uppercase copy
dd if=disk.img bs=512 skip=2048 count=1        # read one 512-byte sector at offset
echo hello | dd conv=ucase                     # HELLO on stdout, summary on stderr
dd if=big of=part bs=4k skip=100 seek=50 count=10
```

## Output summary

On completion (unless `status=none`) dd prints to **stderr**:

```
N+M records in
N+M records out
B bytes copied, T s, R MB/s
```

`N` counts **full** blocks (exactly `ibs`/`obs` bytes) and `M` counts **partial**
blocks. `status=noxfer` drops the third line; `status=none` suppresses all of it.

## Conversions (`conv=`)

| Value | Behaviour |
|---|---|
| `ucase` / `lcase` | Map ASCII case (mutually exclusive). |
| `swab` | Swap each pair of adjacent bytes; a trailing odd byte passes through. |
| `notrunc` | Do not truncate the output file; preserve bytes beyond what is written. |
| `noerror` | Continue after a read error (with `sync`, pad the failed block). |
| `sync` | Pad each short/partial input block with `NUL` up to `ibs`. |
| `sparse` | Seek over all-`NUL` output blocks, creating a hole instead of writing. |

Per-block conversion order: `sync` padding (applied while reading) → `swab` →
`ucase`/`lcase`; `sparse` is realised in the output writer.

## Behavioural notes & caveats

- **`bs=` mode** writes each input block as a single output record (no
  re-aggregation), matching GNU/BSD `dd`. With separate `ibs`/`obs`, reads are
  re-buffered and flushed in `obs`-sized writes, with a final short write.
- **Truncation:** `of=` truncates by default, except when `seek=` is given (the
  leading bytes are preserved) or `conv=notrunc` is set.
- **Non-seekable `skip`/`seek`:** seekability is probed at runtime. On a pipe,
  `skip=` falls back to read-and-discard and `seek=` writes `NUL` padding.
- **`SIGUSR1`** prints current statistics mid-copy on Unix. On Windows this is
  compiled out (`signal_other.go`) and is a documented no-op; the core copy is
  unaffected.
- `iflag`/`oflag` values `direct`, `dsync`, etc. are accepted for compatibility
  but treated as no-ops; only `iflag=fullblock` changes behaviour (it assembles
  complete `ibs` blocks across short reads).

## Source layout

| File | Responsibility |
|---|---|
| `main.go` | Entry point: parse → run → report → exit code. |
| `operands.go` | `Config`, operand parsing, size-suffix parsing, validation. |
| `copy.go` | Block copy engine: skip/seek/count, aggregation, sparse, truncate. |
| `convert.go` | Conversion filters (`swab`, `ucase`, `lcase`, `allZero`). |
| `stats.go` | Atomic counters and summary/transfer-line rendering. |
| `signal_unix.go` / `signal_other.go` | `SIGUSR1` handler (build-tag isolated). |
| `dd_test.go` | Table-driven tests (parsing, engine, conversions, stats, stdio). |
