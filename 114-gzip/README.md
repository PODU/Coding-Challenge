# gzipcc — a from-scratch `gzip`/`gunzip` (Coding Challenge #114)

A production-ready clone of the Unix `gzip`/`gunzip` utility, written in Go.
It produces and consumes the **gzip container format (RFC 1952)** wrapping a
**hand-rolled DEFLATE bitstream (RFC 1951)**.

The DEFLATE core — the LZ77 match finder, the canonical Huffman coders (fixed
*and* dynamic), and the LSB-first bit reader/writer — is implemented by hand.
The standard library `compress/flate` and `compress/gzip` are used **only in the
test suite** as independent oracles to prove two-way interoperability. The only
stdlib primitive used in product code is `hash/crc32` *in tests*; the running
CRC-32 used by the tool is also implemented by hand (`internal/crc32`).

## Build & run

```
go build -o gzipcc .        # build the CLI
go test ./...               # unit + interop + fuzz-seed tests
go vet ./...
```

## Usage

```
gzipcc [OPTION]... [FILE]...

  (default)         compress FILE -> FILE.gz, remove FILE
  -d, --decompress  decompress FILE.gz -> FILE, remove FILE.gz
  -k, --keep        keep (don't delete) input files
  -c, --stdout      write to stdout, keep input
  -f, --force       overwrite existing output / allow tty
  -t, --test        test integrity (verify CRC-32 + ISIZE), no output
  -l, --list        list compressed/uncompressed size and ratio
  -1 .. -9          compression level (default 6); --fast=-1, --best=-9
  -v, --verbose     verbose progress
  -h, --help        usage
  -V, --version     version

With no FILE, or FILE = "-", read stdin and write stdout.
```

Exit codes: `0` success, `1` a file failed (other files still processed),
`2` usage error.

## Architecture

```
main (./main.go)
  └── internal/cli       argument parsing, file/stdin orchestration, exit codes
        └── internal/gzip    RFC 1952 container: header + trailer (CRC-32, ISIZE),
              │              concatenated members, sliding-window output sink
              ├── internal/deflate   RFC 1951 codec
              │     ├── tables.go    length/distance code tables, fixed code lengths
              │     ├── huffman.go   canonical code build (package-merge,
              │     │                length-limited), fixed tables, canonical decoder
              │     ├── writer.go    LZ77 hash-chain finder + lazy matching, tokens
              │     ├── block.go     block-type chooser (stored/fixed/dynamic) + emit
              │     └── reader.go    inflate: stored/fixed/dynamic block parsing
              ├── internal/bitio     LSB-first BitWriter / BitReader
              └── internal/crc32     hand-rolled table-driven IEEE CRC-32
```

### DEFLATE details

- **LZ77**: 32 KiB sliding window, 3-byte hash chains, configurable chain
  length and lazy matching per level (`-1` favours speed, `-9` favours ratio).
  Matches are length 3..258, distance 1..32768.
- **Huffman**: per-block frequencies feed a length-limited (≤15-bit) canonical
  code built with the optimal **package-merge** algorithm. Codes are written
  MSB-first by bit-reversing the canonical code into the LSB-first stream.
- **Block strategy**: each block estimates the bit cost of stored vs fixed vs
  dynamic and emits the cheapest. Incompressible data falls back to stored, so
  output never expands by more than the small stored-block overhead. Blocks are
  bounded so the stored `LEN` field (16-bit) always fits and large files stream.
- **Decoder**: a canonical first-code/offset decoder built from received code
  lengths; back-references copy byte-by-byte so overlapping copies
  (distance < length) work.

### gzip container

- 10-byte header (`1f 8b 08`, FLG, MTIME, XFL, OS=255), optional NUL-terminated
  FNAME on write; on read all optional fields (FEXTRA, FNAME, FCOMMENT, FHCRC)
  are parsed/skipped.
- 8-byte trailer: little-endian CRC-32 of the uncompressed data and ISIZE
  (uncompressed size mod 2³²). Both are **verified** on decompress; a mismatch
  is a hard error with a non-zero exit and no partial output left on disk
  (output is buffered and only written on success).
- Concatenated gzip members are decoded until EOF.

## Tests

`go test ./...` covers:

- **Round-trip identity** across all levels for empty, single-byte, all-same,
  text, source code, repetitive, random, and multi-megabyte multi-block inputs.
- **Interop both ways** with `compress/flate`, `compress/gzip`, and (manually)
  the system `gzip`/`gunzip`.
- **Forced block-type coverage** (stored/fixed/dynamic each round-trips and is
  stdlib-decodable) and a check that the chooser picks `stored` for random data.
- **CRC-32 / ISIZE validation**: corrupt CRC → `ErrChecksum`, corrupt ISIZE →
  `ErrSize`, corrupt payload / truncation → error.
- **CRC-32 unit test** vs `hash/crc32` and the canonical `"123456789"` →
  `0xCBF43926` vector.
- **Fuzz** targets `FuzzRoundTrip` (DEFLATE) and `FuzzGzipRoundTrip` (container,
  cross-checked against the stdlib).

## Caveats

- Compression is not byte-identical to GNU `gzip` (the match-finder heuristics
  differ); output is fully spec-compliant and interoperable in both directions.
- `Compress`/`Decompress` operate on a full member in memory (input buffered,
  output streamed with a 32 KiB window). Extremely large single members use
  memory proportional to the input; multi-gigabyte streaming chunking is out of
  scope for this challenge.
- `--list` reports sizes for a single member by decompressing it (so the ratio
  is exact rather than read from the trailer alone).
