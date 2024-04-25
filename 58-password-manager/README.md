# ccpm — Encrypted Command-Line Password Manager

John Crickett's [Coding Challenge #58](https://codingchallenges.substack.com/p/coding-challenge-58-password-manager).

`ccpm` stores credentials (name, username, password, optional notes) **encrypted
at rest** under a single master password. The master password is never stored:
it is stretched through **Argon2id** into a 32-byte key that
**AES-256-GCM**-encrypts the whole vault. Wrong passwords and tampered files are
rejected cryptographically — there is no plaintext on disk, ever.

## Build

```sh
go build -o ccpm ./cmd/ccpm     # single static binary
```

Requires Go 1.24+. Dependencies: `golang.org/x/crypto/argon2`, `golang.org/x/term`.

## Usage

```
ccpm init    <name>                 Create a new encrypted vault (<name>.ccpm)
ccpm signin  <name>                 Authenticate and open a session
ccpm signout <name>                 End the session
ccpm add     <name> --record ID [--username U] [--password P] [--gen] [--notes N]
ccpm get     <name> --record ID [--show]
ccpm list    <name>
ccpm update  <name> --record ID [--username U] [--password P] [--gen] [--notes N]
ccpm rm      <name> --record ID
ccpm gen     [--length N] [--no-upper] [--no-lower] [--no-digits] [--no-symbols]
```

Common flags: `--vault PATH` (override file location), `--timeout DUR` (session
TTL, e.g. `15m`; `0` disables session caching).

The master password is read **without echo**, or taken from `$CCPM_MASTER` for
scripted / automated use.

### Example

```sh
ccpm init personal                                  # prompts for master pw twice
ccpm add personal --record github --username alice --gen --length 18
ccpm list personal                                  # github
ccpm get  personal --record github --show
ccpm update personal --record github --gen          # rotate the password
ccpm rm   personal --record github
ccpm gen  --length 30                               # standalone generator
```

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | user error (bad flags, unknown record, duplicate, etc.) |
| 2 | authentication failure (wrong master password) |
| 3 | tamper / corruption (bad magic, version, truncation) |

## File format

Binary-framed, versioned, self-describing (`<name>.ccpm`, permission `0600`):

```
MAGIC "CCPM" (4) | VERSION (1) | HEADER_LEN u16 LE (2) |
HEADER JSON {kdf, time, memory, threads, salt} | NONCE (12) | CIPHERTEXT+TAG
```

The header is plaintext (it must be, to derive the key) but holds **no secrets** —
only public KDF parameters and the random salt. It is bound into GCM as
**associated data (AAD)**, so a parameter-downgrade or salt-swap is detected on
open. Writes are atomic: serialize to a temp file in the same directory, `fsync`,
then `rename` over the original, so a crash mid-write can never corrupt an
existing vault.

## Security notes

- **No plaintext at rest.** Only ciphertext plus public KDF parameters and salt.
- **Master password is never stored** — only the Argon2id-derived key, in memory
  and (for the session window) in a `0600` session file.
- **Argon2id** (`time=3`, `memory=64 MiB`, `threads=4`, 32-byte key) is
  memory-hard, defeating commodity GPU/ASIC brute-forcing. A 16-byte random salt,
  unique per vault, defeats rainbow tables and cross-vault correlation. Parameters
  live in the header so they can evolve.
- **AES-256-GCM** provides authenticated encryption — confidentiality *and*
  integrity in one pass. A wrong key (wrong master password) and a tampered file
  both fail the GCM auth-tag check; the tool intentionally does not distinguish
  them beyond a generic failure (no oracle).
- **Nonce discipline:** a fresh 12-byte random nonce on every save, never reused
  under one key (GCM nonce reuse is catastrophic).
- **CSPRNG everywhere** (`crypto/rand`) for salt, nonce, and generated passwords.
  The generator uses rejection sampling (`rand.Int`) to avoid modulo bias and
  guarantees at least one character from each enabled class.
- **File permissions `0600`** on vault and session files.
- **Best-effort memory zeroing** of derived keys and decrypted plaintext after
  use. Acknowledged limitation: Go's garbage collector may copy/relocate memory,
  so this is mitigation, not a guarantee.
- **Session trade-off:** to satisfy "don't retype the master password every
  command", `signin` caches the *derived key* (not the password) in the user's
  temp dir, `0600`, with a hard expiry (default 15 min). Mitigations: short TTL,
  `signout` deletes it, and `--timeout 0` disables caching entirely. This is a
  documented usability-vs-security choice.

## Tests

```sh
go test ./...
```

Coverage includes: encrypt→decrypt round-trip (empty/small/large), wrong master
password fails with a clear error and no panic, single-byte tamper of
ciphertext/nonce/header/AAD → auth failure, nonce uniqueness, KDF determinism,
file-codec round-trip and corruption cases, full CRUD, persistence across a fresh
process invocation (temp vault), generator (length, charset honoring, per-class
guarantees, entropy, modulo-bias sanity, empty-alphabet rejection), session
start/load/expiry/signout, and an end-to-end CLI golden path asserting exit codes.

## Project layout

```
cmd/ccpm/main.go         CLI entry + subcommand dispatch
internal/crypto/         Argon2id KDF + AES-256-GCM seal/open
internal/vault/          file codec + Record/Vault CRUD + persistence
internal/session/        session key cache + timeout
internal/genpass/        password generator
```

## Caveats

- Whole-vault seal is O(n) per write — fine for a personal manager, not a
  multi-thousand-record store.
- Last-writer-wins on concurrent writes (atomic rename prevents *corruption*, not
  lost updates).
- Memory zeroing is best-effort (Go GC limitation, above).
- The session file holds the derived key on disk for the TTL window — disable with
  `--timeout 0` if that trade-off is unacceptable.
