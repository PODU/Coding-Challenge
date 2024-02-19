# passcrack — Coding Challenge #49: Build Your Own Password Cracker

An educational command-line password cracker, in Go. Given one or more password
*hashes*, it recovers the plaintext by hashing candidate passwords and comparing
digests — via **dictionary**, **brute-force**, or **rainbow-table** strategies,
across **MD5 / SHA-1 / SHA-256 / SHA-512**, with optional salting and a
concurrent worker pool.

> Source challenge: John Crickett's Coding Challenges —
> <https://codingchallenges.substack.com/p/coding-challenge-49-password-cracker>

---

## ⚠️ Ethical & legal use only — read this first

**This is a teaching tool. Use it only on hashes you own or are explicitly
authorized to test** — your own accounts, hashes you generated, your CTF/lab
boxes, or a penetration test with **written** authorization. Cracking
third-party credentials without permission is illegal in most jurisdictions and
against the spirit of this challenge. `passcrack` prints this notice on every run.

**The real lesson:** fast, *unsalted* hashes like MD5 and SHA-1 fall to a laptop
in seconds. That is exactly *why you must never store passwords this way.* Real
systems use a **slow, salted password KDF** — bcrypt, scrypt, Argon2, or
PBKDF2 — which this tool deliberately does **not** try to crack. Per-user random
salts and KDF hash formats (`$2b$...`, `$argon2...`) are out of scope by design;
they are the defense this exercise teaches you to respect.

---

## Build & test

```sh
go build -o passcrack .       # single static binary, stdlib only
go test ./...                 # full deterministic, offline test suite
go vet ./...
```

> **Note on `-race`:** the suite is race-safe and intended to be run with
> `go test ./... -race` in CI. The race detector needs a 64-bit C toolchain
> (cgo); on a machine with only a 32-bit MinGW gcc the `-race` build fails with
> `sorry, unimplemented: 64-bit mode not compiled in`. Run `-race` on Linux/macOS
> or with a 64-bit gcc/clang installed.

## Usage

```
passcrack [flags] [TARGET_HASH]

  -a, --algo string      md5 | sha1 | sha256 | sha512 | auto   (default "md5")
  -m, --mode string      dict | brute | rainbow                (default "dict")
  -w, --wordlist string  wordlist path, or "-" for stdin
      --hashes-file str  file of target hashes, one per line
      --charset string   brute-force alphabet                  (default a-z0-9)
      --min int          brute-force min length                (default 1)
      --max int          brute-force max length                (default 6)
      --salt string      optional salt value
      --salt-mode str    prefix | suffix                       (default "prefix")
      --workers int      worker goroutines                     (default NumCPU)
      --rainbow-file s   rainbow table path (build/lookup)
      --build-rainbow    build the table instead of looking up
  -q, --quiet            suppress the stderr banner/progress
  -h, --help

Exit codes:  0 = all cracked   1 = some uncracked   2 = usage/IO error
```

Results (`hash:plaintext`) go to **stdout** (clean for piping); the banner,
progress, and `UNCRACKED <hash>` lines go to **stderr**.

### Examples

```sh
# Dictionary crack one MD5 from a wordlist
passcrack -a md5 -m dict -w testdata/words.txt 2ab96390c7dbe3439de74d0c9b0b1767
# -> 2ab96390c7dbe3439de74d0c9b0b1767:hunter2

# Brute force the challenge's published MD5 vector ("PASS")
passcrack -m brute --charset ABCDEFGHIJKLMNOPQRSTUVWXYZ --max 4 7a95bf926a0333f57705aeac07a362a2
# -> 7a95bf926a0333f57705aeac07a362a2:PASS

# Many hashes from a file, SHA-256, salted suffix
passcrack -a sha256 --salt s3cr3t --salt-mode suffix --hashes-file hashes.txt -w words.txt

# Auto-detect the algorithm from the digest length
passcrack -a auto -w words.txt ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad

# Build a rainbow table, then do instant lookups against it
passcrack -m rainbow --build-rainbow -w words.txt --rainbow-file rt.tsv
passcrack -m rainbow --rainbow-file rt.tsv 2ab96390c7dbe3439de74d0c9b0b1767
```

## How it works

```
CLI flags ─▶ cmd/main ─▶ engine (worker pool) ◀─ candidates ◀─ producer
                              │                                 (wordlist |
                              ▼                                  brute | rainbow)
                         stdout: hash:plaintext
                         stderr: progress, UNCRACKED
```

| Package      | Responsibility |
|--------------|----------------|
| `hashreg`    | Algorithm registry behind one `Algo` interface; `Detect(hexLen)` for `-a auto`. Add a new algorithm in one place. |
| `wordlist`   | Streaming, constant-memory reader (`bufio.Scanner`); trims and skips blanks. |
| `bruteforce` | Odometer-style candidate generator over a charset, length `min..max`; context-cancellable. |
| `rainbow`    | Build a `hash<TAB>plaintext` table, then look hashes up instantly. |
| `saltutil`   | Apply a salt as `prefix` (`H(salt+pw)`) or `suffix` (`H(pw+salt)`). |
| `engine`     | Concurrent worker pool; read-only target set; cancels all work the instant every target is cracked. |
| `main`       | Flag parsing/validation, ethics banner, producer selection, exit codes. |

### Concurrency

One producer goroutine feeds a candidate channel consumed by `--workers`
hashing goroutines. The target set is a read-only `map[digest]→{}` during
cracking, so the hot path needs no locking. When the last target is found the
engine cancels the context, the producer stops, channels drain, and the
`WaitGroup` joins — no goroutine leaks, bounded memory regardless of input size.

## Test coverage

`go test ./...` runs offline with committed fixtures:

- **Algorithm correctness** — golden vectors (e.g. `MD5("abc")`,
  `SHA-256("abc")`) and a randomized cross-check against the Go stdlib.
- **Dictionary crack** — recovers `MD5("hunter2")` from `testdata/words.txt`;
  negative test leaves an absent hash `UNCRACKED`.
- **Brute-force** — recovers `MD5("cab")` over charset `abc`; generator emits
  exactly `Σ len^k` candidates and is cancellable.
- **Challenge parity** — recovers the published MD5 brute vectors
  `7a95bf926a0333f57705aeac07a362a2` (`"PASS"`) and
  `08054846bbc9933fd0395f8be516a9f9` (`"CODE"`).
- **Salt** — a salted hash cracks only when the matching `--salt`/`--salt-mode`
  is supplied; a plain attack fails.
- **Multiple hashes** — two targets in one pass with early exit.
- **Rainbow** — build then lookup (hit and miss), including a salted table.
- **CLI / exit codes** — table-driven `0/1/2` checks, pipe-clean stdout,
  `--hashes-file`, `-a auto`, and rainbow build+lookup end to end.
- **Worker-count invariance** — identical results for `--workers` 1..8.
