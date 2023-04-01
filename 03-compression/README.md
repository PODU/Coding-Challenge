# huff — Huffman compression tool

Coding Challenge #3. A lossless command-line compressor/decompressor built on
[Huffman coding](https://en.wikipedia.org/wiki/Huffman_coding): it counts byte
frequencies, builds a min-heap Huffman tree, derives prefix-free codes, and
MSB-first bit-packs the payload into a self-describing container. Decompression
reverses the process for a **byte-for-byte identical** round-trip on any input
(text or binary).

## Layout

```
03-compression/
  go.mod
  cmd/huff/main.go            CLI entry point, arg parsing, exit codes
  internal/huffman/
    freq.go                   frequency counting ([256]uint64)
    tree.go                   node, container/heap min-heap, tree build
    code.go                   prefix-code table from tree traversal
    bitio.go                  BitWriter / BitReader (MSB-first)
    header.go                 HUF1 container header read/write + validation
    compress.go               compress orchestration + Stats
    decompress.go             decompress orchestration
    *_test.go                 unit, round-trip, property, header, corpus tests
  testdata/                   (optional) les_miserables.txt — fetched, gitignored
```

## Build

Requires Go 1.24+.

```sh
go build ./...                 # build everything
go build -o huff ./cmd/huff    # produce the ./huff binary (huff.exe on Windows)
```

## Run

```
huff compress   [-o <output>] [-v] [-f] <input>
huff decompress [-o <output>] [-v] [-f] <input>
huff -h | --help
```

Flags:

- `-o, --output <path>` — explicit output path. Default for `compress` is
  `<input>.huff`; for `decompress` it strips a trailing `.huff` (or appends
  `.out`).
- `-v, --verbose` — report sizes and compression ratio to stderr.
- `-f, --force` — overwrite the output file if it already exists.
- `-h, --help` — usage.

Exit codes: `0` success · `1` usage error · `2` I/O error · `3` corrupt /
invalid compressed input (bad magic or truncated header/payload).

### Examples

```sh
# Compress a book and see the ratio
$ huff compress -v les_miserables.txt
compressed 3,290,880 -> 1,856,402 bytes (56.4%, ratio 1.77x), 95 distinct symbols

# Decompress (default output strips the .huff suffix)
$ huff decompress les_miserables.txt.huff -o restored.txt

# Verify the round-trip is byte-for-byte identical
$ cmp les_miserables.txt restored.txt && echo identical
identical

# Explicit output + overwrite
$ huff compress -f -o out.huff input.bin
```

Compression and decompression are atomic: output is written to a temporary file
in the destination directory and renamed into place only on success, so a
failure never leaves a partial/garbage file behind.

## Test

```sh
go test ./...              # full suite
go test ./... -v           # verbose
go test ./... -run Round   # round-trip tests only
go test ./internal/huffman -bench . -benchmem   # compress/decompress benchmarks
```

The suite covers:

- **Round-trip correctness** — fixtures (empty, single byte, repeated symbol,
  all-256-bytes, ASCII, control chars), a 50 KB random-binary file, and
  property-based fuzzing of 2000 random byte slices (`testing/quick`).
- **Unit tests** — frequency counting, single-symbol 1-bit code, prefix-free
  property (trie check), deterministic tree shape, `BitWriter`/`BitReader`
  across non-byte-aligned lengths, header serialize/parse for 0/1/2/256
  symbols.
- **Robustness** — rejects bad magic, truncated headers, and truncated payloads
  with `ErrBadMagic` / `ErrCorrupt`.
- **Compression ratio** — asserts output is strictly smaller than input on
  English-like text.
- **CLI integration** — end-to-end compress→decompress, default output paths,
  overwrite refusal without `-f`, and all four exit codes.
- **Benchmarks** — compress/decompress at ~3.2 MB (Les Mis scale).

### Canonical corpus (optional)

To run the *Les Misérables* validation test (asserts `t` = 223,000 and
`X` = 333 occurrences, plus a strictly-smaller compressed size), download the
plain-text from Project Gutenberg and save it as
`testdata/les_miserables.txt`:

```sh
mkdir -p testdata
curl -L -o testdata/les_miserables.txt https://www.gutenberg.org/files/135/135-0.txt
go test ./internal/huffman -run LesMiserables -v
```

The file is gitignored and the test skips cleanly when it is absent, so the
suite passes offline.

## File format (HUF1)

```
Offset  Size       Field         Description
------  ---------  ------------  -------------------------------------------
0       4 bytes    MAGIC         ASCII "HUF1"
4       8 bytes    SYMBOL_COUNT  uint64 BE — number of original bytes
12      2 bytes    TABLE_LEN     uint16 BE — distinct symbols (0..256)
14      TABLE_LEN  ENTRIES       repeated { symbol:1, freq:uint64 BE }
...     N bytes    PAYLOAD       MSB-first packed Huffman bitstream
```

The frequency table (not the serialized tree) is stored; the decoder rebuilds
the identical tree using the same construction algorithm and stable tie-break,
so the codes match exactly. `SYMBOL_COUNT` is the authoritative decode stop
condition, which is why zero-padding the final byte is safe. Empty files are a
14-byte header with no entries and no payload; single-symbol files store one
table entry and emit the lone symbol `SYMBOL_COUNT` times without reading the
(padding-only) payload.
