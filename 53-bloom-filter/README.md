# ccspellcheck — Build Your Own Bloom Filter

[Coding Challenge #53](https://codingchallenges.substack.com/p/coding-challenge-53-bloom-filter).
A from-scratch [Bloom filter](https://en.wikipedia.org/wiki/Bloom_filter) in Go,
wrapped in a spell-checker CLI. A Bloom filter answers set-membership queries
with **no false negatives** and a tunable, bounded **false-positive rate** — so
a ~2.4 MB dictionary compresses to a few hundred KB while still answering "is
this word in the dictionary?" in O(k) time per query.

## Build

```sh
go build -o ccspellcheck ./cmd/ccspellcheck
```

Requires Go 1.24+. No third-party dependencies.

## Usage

Build a model from a dictionary (one word per line):

```sh
ccspellcheck -build /usr/share/dict/words            # -> words.bf
ccspellcheck -build dict.txt -o my.bf                # custom output path
ccspellcheck -build dict.txt -p 0.001                # custom false-positive target
```

Check words (prints those **probably misspelled** — i.e. absent from the model):

```sh
ccspellcheck hi hello wrod                            # loads words.bf
ccspellcheck -f my.bf hi hello wrod                   # custom model file
```

### Flags

| Flag           | Meaning                                                       | Default     |
|----------------|--------------------------------------------------------------|-------------|
| `-build <path>`| Dictionary word list → build mode                            | —           |
| `-o <path>`    | Output model path (build mode)                               | `words.bf`  |
| `-f <path>`    | Model file to load (check mode)                             | `words.bf`  |
| `-p <float>`   | Target false-positive probability (build mode), in (0, 1)   | `0.01`      |
| `-h`, `--help` | Usage                                                        | —           |

### Exit codes (check mode)

Grep-style: the exit status signals whether misspellings were found.

| Code | Meaning                              |
|------|--------------------------------------|
| `0`  | All words probably correct           |
| `1`  | One or more words flagged misspelled |
| `2`  | Usage or I/O error                   |

## How it works

- **Bit array.** A `[]byte` of `ceil(m/8)` bytes; bit `i` lives at `bits[i>>3]`
  with mask `1 << (i&7)`.
- **Sizing.** From expected item count `n` and target probability `p`:
  `m = ceil(-(n·ln p) / (ln 2)²)` and `k = round((m/n)·ln 2)` (clamped to ≥ 1).
  Computed once at build time and stored in the file header — load never
  recomputes, so build and check behave identically.
- **k hashes via double hashing.** One 64-bit FNV-1a hash is split into two
  32-bit halves `h1`, `h2` (with `h2` forced odd); index `i` is `(h1 + i·h2) mod m`
  ([Kirsch–Mitzenmacher](https://www.eecs.harvard.edu/~michaelm/postscripts/rsa2008.pdf)).
- **Add / Contains.** `Add` sets the `k` bits; `Contains` returns `false` the
  moment any indexed bit is 0 (definitely absent), else `true` (probably present).
- **Normalization.** Words are trimmed and lowercased on **both** build and
  check, so `Hello` and `hello` agree. Apostrophes/hyphens are kept as-is.
  Hashing is over UTF-8 bytes, so non-ASCII words work.

## File format (`*.bf`)

All multi-byte integers are **big-endian**:

| Offset | Size | Field   | Value                       |
|--------|------|---------|-----------------------------|
| 0      | 4    | magic   | ASCII `CCBF`                |
| 4      | 2    | version | uint16 (currently `1`)      |
| 6      | 2    | k       | uint16 (number of hashes)   |
| 8      | 4    | m       | uint32 (number of bits)     |
| 12     | …    | bits    | `ceil(m/8)` raw bitset bytes |

Load validates the magic and version before trusting the rest of the file;
bad magic, an unsupported version, or a truncated bit array each produce a
distinct, descriptive error (never a panic).

## Project layout

```
53-bloom-filter/
├── go.mod                       module ccspellcheck
├── cmd/ccspellcheck/main.go     CLI: flag parsing, build/check dispatch
└── internal/bloom/
    ├── bloom.go                 BloomFilter: New, Add, Contains, Optimal m/k
    ├── bloom_test.go            no-false-negatives, FP-rate, m/k math, unicode
    ├── codec.go                 WriteTo / ReadFrom (CCBF binary format)
    └── codec_test.go            round-trip + header-validation tests
```

## Tests

```sh
go test ./...                    # all tests
go test ./... -bench=. -benchmem # include Add/Contains benchmarks
```

Coverage includes: the no-false-negatives guarantee across several `(n, p)`
sizes; a measured false-positive rate within tolerance of target `p` over a
200k-sample query set; `m`/`k` formula values vs. hand-computed references;
persistence round-trip (identical `m`/`k`/bitset and identical answers); header
validation (bad magic, unsupported version, truncated header/bit array); and a
CLI build→persist→check round-trip with exit-code assertions.
