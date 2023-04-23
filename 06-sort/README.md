# ccsort — Build Your Own `sort`

A production-grade clone of the Unix `sort` utility, written in safe, idiomatic
Rust. This is the implementation of [John Crickett's Coding Challenge #6][cc].

`ccsort` does lexicographic (byte-accurate, `LC_ALL=C`) sorting with five
selectable algorithms, GNU-style flags, `-u` uniqueness, `-R` random hash-sort
(equal keys grouped), and a **memory-bounded external merge sort** for inputs
larger than RAM.

[cc]: https://codingchallenges.substack.com/p/coding-challenge-6

---

## Highlights

- **Five sorting algorithms**, each implemented from scratch: `radix` (MSD,
  byte-wise), `merge` (stable), `quick` (median-of-three + insertion cutoff),
  `heap`, plus the standard-library `default` baseline. Pick one with
  `--algorithm`.
- **Three+ data structures**: a dynamic line buffer (`Vec`), a hash set (for the
  `-u` hash-set strategy and `-R` keying), and a binary min-heap (the k-way
  external merge).
- **Byte-accurate ordering** — operates on raw `Vec<u8>`, never panics on
  invalid UTF-8 or binary input, and matches `LC_ALL=C` sort order exactly.
- **Memory-bounded** — set a budget with `-S`; beyond it, input is sorted in
  bounded chunks spilled to temp files and combined with a k-way merge.
- **Zero dependencies** — argument parsing, hashing, and the merge heap are all
  hand-rolled on top of `std`, so `cargo build` works fully offline.

---

## Build, run, test

This is a standard Cargo project.

```sh
# Build (optimized binary at target/release/ccsort)
cargo build --release

# Run the full test suite (unit + integration + property tests)
cargo test

# Run the binary
./target/release/ccsort fixtures/words.txt

# Compare all five algorithms on synthetic data
cargo run --release --example bench
```

> **Toolchain note:** this repository was authored in an environment **without a
> Rust toolchain installed**, so `cargo build`/`cargo test` could not be run
> here. The code is dependency-free and written to compile on stable Rust
> (edition 2021); run the commands above locally to build and verify.

---

## Usage

```
ccsort [OPTIONS] [FILE...]
```

Reads `FILE`(s) — or stdin if none, or `-` — sorts the lines, and writes to
stdout (or `-o FILE`).

### Ordering
| Flag | Long | Meaning |
|------|------|---------|
| `-n` | `--numeric-sort` | Compare by leading numeric value |
| `-r` | `--reverse` | Reverse the comparison (descending) |
| `-R` | `--random-sort` | Sort by a random hash; equal lines group |
| `-f` | `--ignore-case` | Fold case before comparing |
| `-b` | `--ignore-leading-blanks` | Ignore leading blanks in the key |

### Uniqueness / check
| Flag | Long | Meaning |
|------|------|---------|
| `-u` | `--unique` | Output only the first of each equal-key run |
| `-c` | `--check[=quiet]` | Check whether input is sorted; do not sort |
| `-C` | | Like `-c` but status only, no diagnostic |

### Algorithm
| Flag | Meaning |
|------|---------|
| `--algorithm <A>` | `radix \| merge \| quick \| heap \| default` (default: `default`) |
| `--sort <WORD>` | GNU shim: `random`, `numeric`, or an algorithm name |
| `--random-source <SEED>` | Seed for `-R` (reproducible runs) |

### I/O & resources
| Flag | Long | Meaning |
|------|------|---------|
| `-o` | `--output <FILE>` | Write to FILE (in-place safe: `-o f f`) |
| `-S` | `--buffer-size <SIZE>` | Memory budget before external sort (`64M`, `1G`, ...) |
|      | `--temp-dir <DIR>` | Directory for external-sort run files |
| `-h` | `--help` | Print help |
| `-V` | `--version` | Print version |

`-k`/`-t` (key fields) are recognised but **not implemented** in this build;
using them is a clean error.

### Exit status
- `0` — success (or, with `-c`, the input was sorted)
- `1` — with `-c`/`-C`: the input was **not** sorted
- `2` — an error occurred (bad option, I/O failure)

### Examples
```sh
ccsort fixtures/words.txt                 # lexicographic sort
ccsort -u fixtures/words.txt              # unique lines
ccsort -n -r fixtures/numbers.txt         # numeric, descending
ccsort --algorithm radix -u fixtures/words.txt
ccsort -R --random-source 42 fixtures/words.txt   # reproducible random sort
ccsort -c sorted.txt; echo $?             # check sortedness
cat a.txt b.txt | ccsort -o merged.txt    # merge two files, write to file
ccsort -S 16M huge.txt                    # force external merge sort
```

---

## Generating test data (Challenge Step 0)

The challenge suggests a large public-domain book and a word list with many
duplicates. Small samples live in [`fixtures/`](./fixtures). To build your own
from a Project Gutenberg text:

```sh
# Download a book as test.txt, then tokenise into one word per line:
tr -cs 'A-Za-z' '\n' < test.txt | grep -v '^$' > words.txt

# words.txt now has many duplicates — great for exercising -u and -R:
ccsort -u words.txt | wc -l        # number of distinct words
ccsort -R --random-source 1 words.txt | head
```

---

## Design notes

### The five sorts and how they compose with `-u`

| Algorithm | Stable | In-place | Best fit here | `-u` interaction |
|-----------|--------|----------|---------------|------------------|
| **radix** (MSD, byte) | yes | no (buckets) | string keys, fixed alphabet | groups equal keys; dedup is a cheap adjacent pass |
| **merge** | yes | no (O(n) aux) | predictable; basis of external sort | adjacent dedup after the pass |
| **quick** | no | yes | low memory, fast average | needs the post-pass dedup |
| **heap** | no | yes | O(n log n) worst case, bounded memory | needs the post-pass dedup |
| **default** (`sort_unstable`) | no | yes | baseline / benchmark | needs the post-pass dedup |

`-u` is implemented as **sort-then-dedup-adjacent** in production: once sorted,
equal keys are neighbours, so one linear pass removes duplicates in `O(1)` extra
space — and it works inside the external merge too. The alternative,
**dedup-while-inserting with a hash set**, is also implemented
([`unique::dedup_hashset`]) and unit-tested against the adjacent strategy; it can
shrink the working set when duplicates are dense but costs `O(n)` memory and
cannot run in a memory-bounded external sort. That trade-off is exactly what the
challenge asks you to evaluate.

### One key, every mode (the Schwartzian transform)

Each line is paired with a precomputed, **order-preserving** comparison key
([`key::build_key`]). Plain `[u8]` comparison of two keys yields the desired
ascending order for *every* mode:

- **lexical** — the key is the (optionally blank-stripped / case-folded) bytes.
- **numeric** (`-n`) — the leading value is parsed to `f64` and encoded into 8
  bytes whose unsigned byte order equals numeric order (sign-bit flip trick).
- **random** (`-R`) — the key is the big-endian seeded hash of the line, so
  identical lines share a key and stay grouped.

Because the key is always a byte string, byte-wise **radix sort just works** for
numeric and random keys too, and a single comparator drives all five algorithms.

### External merge sort (files larger than RAM)

The engine reads lines lazily, tracking an approximate byte budget (`-S`). While
the data fits, it stays on a fast in-memory path. The moment the budget would be
exceeded it sorts the current chunk with the selected algorithm, spills it to a
temp run file, and switches to the external path; remaining input is processed in
bounded chunks the same way. Finally a **k-way merge** opens all runs and uses a
binary min-heap of run heads to emit the global order, applying `-u` and `-R`/`-n`
semantics during the merge so memory stays bounded throughout. Temp files (and
their directory) are removed when the run set is dropped — including on error.

### Deviations from GNU `sort` (documented on purpose)

- **No last-resort comparison.** When two keys compare equal, GNU falls back to
  comparing whole raw lines. `ccsort` does not: equal-key lines keep input order
  under the stable algorithms (`radix`/`merge`) or an arbitrary order under the
  unstable ones. This only affects the *order among lines that share a key*
  (possible with `-b`/`-f`/`-n`), never which lines appear, and it keeps all five
  algorithms mutually consistent. Use `--algorithm merge` for fully stable output.
- **Numeric parsing has no exponent.** `-n` reads `[blanks][sign]digits[.digits]`
  and stops at anything else, so `1e3` parses as `1` — matching GNU `sort -n`.
  Values are compared as `f64`.
- **`-R` hash** is a built-in seeded FNV-1a (no `--random-source` file support;
  a numeric or textual seed is accepted and is reproducible).

---

## Project layout

```
src/
  main.rs        entry point, exit-code mapping
  lib.rs         module wiring + exit codes
  cli.rs         hand-rolled argument parser (+ size parsing, help text)
  config.rs      Config / Algorithm types
  reader.rs      streaming multi-source line reader (Vec<u8> lines)
  key.rs         order-preserving key extraction (lexical / numeric / random)
  cmp.rs         the active comparator (reverse-aware)
  record.rs      Record { line, key }
  algo/
    mod.rs       Sorter trait + dispatch + in-place permutation helper
    radix.rs     MSD byte-wise radix sort
    merge.rs     stable top-down merge sort
    quick.rs     median-of-three quicksort + insertion cutoff
    heap.rs      binary heapsort
    default.rs   std sort_unstable baseline
  unique.rs      -u strategies (adjacent dedup + hash-set dedup)
  random.rs      seeded hashing + seed resolution for -R
  external.rs    spill + k-way merge (TempRuns) for memory-bounded sorting
  output.rs      stdout / file output with atomic temp+rename (in-place safe)
  check.rs       -c / -C verification pass
  engine.rs      orchestration: read -> sort/spill -> dedup/merge -> write
tests/
  cli.rs         end-to-end tests driving the compiled binary
  algorithms.rs  property-style invariants over the library API
examples/
  bench.rs       five-algorithm timing harness
fixtures/        small sample inputs (words.txt, numbers.txt)
```

## Testing

- **Unit tests** live next to each module (`cargo test`): every algorithm
  against shared fixtures (empty, single, sorted, reverse, all-equal, duplicates)
  asserting non-decreasing output and multiset/permutation equality; comparator,
  key extraction (including the numeric edge cases), both `-u` strategies, random
  reproducibility, the external merge, in-place output, and reader edge cases.
- **`tests/cli.rs`** drives the real binary: ordering across all five algorithms,
  `-u`/`-r`/`-n` and combinations, `-c` exit codes, `-o` (incl. in-place), stdin
  and multi-file concatenation, `-R` reproducibility + grouping, the external
  path forced by a tiny `-S`, error exit codes, and binary-byte fuzzing.
- **`tests/algorithms.rs`** are property-style: for many pseudo-random inputs,
  every algorithm yields a sorted permutation, `-u` leaves no adjacent equal
  keys, and `-R` with a fixed seed is a deterministic permutation.

## License

MIT.
