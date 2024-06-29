# cczipcrack — ZIP File Cracker (Coding Challenge #66)

A command-line tool that recovers the password of a password-protected ZIP
archive via a **dictionary attack** or a **brute-force attack**, verifying each
candidate with a real decrypt + CRC-32 check, and spreading the work across all
CPU cores. It supports legacy **ZipCrypto** (Python stdlib `zipfile`) and, when
[`pyzipper`](https://pypi.org/project/pyzipper/) is installed, **WinZip AES**.

> John Crickett's Coding Challenges — Challenge #66
> https://codingchallenges.substack.com/p/coding-challenge-66-zip-file-cracker

---

## ⚠️ Ethical Use Notice

This is an **educational** project. Its purpose is to demonstrate *why* the
legacy ZIP encryption scheme (PKWARE "ZipCrypto") is cryptographically weak and
trivially brute-forceable, and to teach concurrency, file-format parsing, and
password-recovery mechanics.

**Only run this tool against ZIP files you own or are explicitly authorized to
test.** Using it to access data you do not own or have permission to access is
illegal in most jurisdictions and is not condoned by the author. The test suite
ships with fixture archives whose password is known (`test`), so every feature
can be demonstrated without touching third-party data.

---

## Install

Requires **Python 3.12+**.

```bash
# Core (ZipCrypto only):
pip install -e .

# With WinZip AES support and test deps:
pip install -e ".[dev]"      # or ".[aes]" for just AES
```

This installs a console script `cczipcrack` and enables `python -m cczipcrack`.

---

## Usage

```
cczipcrack [GLOBAL] <zipfile> [MODE]

Modes (choose one; default = validate-only):
  -dict [-w PATH]         Dictionary attack (default wordlist: wordlists/smaller.txt).
  -brute                  Brute-force attack. Use with -min / -max.
    -min=N                Minimum candidate length (default 1).
    -max=N                Maximum candidate length (default 4).
    --charset=STR         Characters to combine (default a-z).

Global options:
  -j, --jobs N            Worker processes (default: os.cpu_count()).
  --recurse               After success, recurse into a nested ZIP (extension).
  -q, --quiet             Suppress progress; print only the result line.
  --version               Show version.
```

### Examples (challenge transcript)

```bash
cczipcrack cctest.zip                       # -> Has zip headers
cczipcrack cc.txt                           # -> Does not have zip headers
cczipcrack -dict cctest.zip                 # -> Password found: test
cczipcrack -brute -min=1 -max=4 cctest.zip  # -> Password found: test
```

**Exit codes:** `0` password found / valid zip · `1` password not found ·
`2` bad usage, unreadable input, or non-ZIP input.

### Wordlist

The default dictionary path is `wordlists/smaller.txt`. The CrackStation
"Smaller Wordlist" is **not committed** (size/licensing) — download it yourself
and point `-w` at it, or use any newline-delimited wordlist:

```bash
cczipcrack -dict -w /path/to/rockyou.txt secret.zip
```

Wordlists are streamed lazily, so multi-hundred-megabyte lists use constant
memory.

---

## How it works

ZIP **metadata is not encrypted** even when file *contents* are. cczipcrack
reads the central directory without a password, detects the encryption scheme,
and picks the **smallest encrypted entry** as a cheap verification target. Each
candidate password is checked by actually decrypting that entry — a wrong
password fails the 12-byte check byte, the stored CRC-32, or decompression; a
right one returns clean bytes. Verification is CPU-bound, so candidates are
chunked and farmed out to a `multiprocessing.Pool` (one worker per core) with
early exit on the first hit.

| Module | Responsibility |
|--------|----------------|
| `zip_reader.py` | Header validation (`is_zip`), scheme detection, target pick. |
| `verifier.py`   | Decrypt-and-CRC a candidate; ZipCrypto (stdlib) / AES (pyzipper). |
| `candidates.py` | Lazy wordlist + brute-force generators. |
| `cracker.py`    | Multi-core orchestration with early exit. |
| `reporter.py`   | The exact challenge output strings. |
| `cli.py`        | argparse front door, mode dispatch, exit codes. |

---

## Tests

```bash
python -m pytest
```

The suite is deterministic and self-contained (no network, no external `zip`
binary). Because neither the stdlib nor pyzipper can *write* legacy ZipCrypto,
the fixture archive is produced by a tiny in-repo PKWARE-cipher writer
(`tests/_zipcrypto_writer.py`) that yields a genuine, stdlib-readable encrypted
ZIP with password `test`. AES tests are automatically skipped if `pyzipper` is
not installed.

Coverage: header validation, scheme detection, the verifier (right/wrong/reuse),
dictionary success & miss, brute-force success & miss, single- vs multi-worker
parity, the not-encrypted branch, AES (gated), and CLI transcript strings + exit
codes (in-process and via `subprocess`).

---

## License

MIT. Educational use; see the Ethical Use Notice above.
