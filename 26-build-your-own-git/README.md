# gg — Build Your Own Git Client

A small, **Git-compatible** version control client written in Go with zero
third-party dependencies. It reads and writes the real on-disk Git format —
loose objects, the v2 index, tree/commit objects, and refs — so repositories it
creates interoperate with the canonical `git` binary: `git status`, `git log`,
and `git fsck --strict` all work against repositories produced by `gg`, and
`gg` reads repositories produced by `git`.

This is John Crickett's [Coding Challenge #26](https://codingchallenges.substack.com/p/coding-challenge-26-build-your-own).

## Features

| Command | Description |
|---------|-------------|
| `gg init [<dir>]` | Create a `.git` layout (HEAD, config, objects, refs/…). Re-init is benign. |
| `gg hash-object [-w] [-t <type>] <file>` | Print the SHA-1; with `-w`, write the loose object. |
| `gg cat-file (-t \| -p \| -s) <oid>` | Inspect an object's type, content, or size. |
| `gg add <pathspec>...` | Stage files: write blobs and insert/update index entries (dir recursion + basic `.gitignore`). |
| `gg status [--porcelain]` | Show staged (vs HEAD), unstaged (vs index), and untracked files. |
| `gg commit -m <msg> [--author "Name <email>"]` | Write tree(s) + commit object, update the branch ref. |
| `gg log [-n <count>]` | Walk commit ancestry from HEAD. |
| `gg diff [<pathspec>...]` | Unified diff of the working tree vs the index. |
| `gg push <remote-url> [<branch>]` | Push over smart HTTP (`git-receive-pack`) with a packfile. |

**Exit codes:** `0` success, `1` runtime error, `2` usage error.

## Build & test

```sh
go build ./cmd/gg          # produces the gg binary
go test ./...              # offline, hermetic test suite (no network)
go vet ./...
```

Cross-check against the real `git` binary (skipped automatically if `git` is
not on `PATH`):

```sh
go test -tags gitcompat ./cmd/gg/
```

These tests assert that a repo built entirely by `gg` passes `git fsck --strict`,
that `git log`/`git status` agree, and that `gg` can read a `git`-created repo.

## Format notes

- **Loose object:** `zlib( "<type> <size>\0" + payload )` at
  `.git/objects/<oid[:2]>/<oid[2:]>`. The OID is the SHA-1 of the uncompressed
  header+payload (e.g. blob of `"hello\n"` →
  `ce013625030ba8dba906f756967f9e9ca394464a`).
- **Tree:** sorted `"<mode> <name>\0" + 20-byte-raw-oid`; tree entries sort as if
  their name had a trailing `/` (so `foo.txt` sorts before the `foo/` tree).
- **Index (DIRC v2):** 12-byte header, fixed 62-byte per-entry stat block,
  16-bit flags, NUL-padded path to 8-byte alignment, trailing SHA-1 over the
  whole file. On Windows, `dev`/`ino`/`uid`/`gid` are written as `0` (Git
  tolerates and refreshes them).
- **Packfile (push):** `PACK` v2, varint object headers, zlib payloads, no
  deltas, 20-byte SHA-1 trailer.

## Authentication for `push`

Push uses HTTPS basic auth. Credentials come from (in order):

1. A `https://<user>:<token>@host/...` URL, or
2. The `GG_TOKEN` (and optional `GG_USER`, default `x-access-token`) environment
   variables.

Tokens are never logged. For GitHub, use a Personal Access Token as the token.

```sh
export GG_TOKEN=ghp_xxx
gg push https://github.com/you/repo.git main
```

## Identity for `commit`

Author/committer identity is resolved from `GG_AUTHOR_NAME` / `GG_AUTHOR_EMAIL`,
then the `[user]` section of `.git/config`, then a built-in fallback.

## Layout

```
cmd/gg/main.go              CLI dispatch + exit codes
internal/object/            blob/tree/commit codec, SHA-1, zlib loose I/O, revwalk
internal/index/             DIRC v2 read/write
internal/repo/              locate/create .git, HEAD & refs
internal/ignore/            minimal .gitignore matching
internal/status/            HEAD <-> index <-> worktree classification
internal/diff/              LCS unified diff
internal/transport/         packfile build/parse + smart-HTTP receive-pack
```

## Caveats

- Push sends whole objects (no delta compression) — valid but larger packs.
- `.gitignore` support is the documented subset (literals, `*`/`?` globs,
  anchored and directory patterns, negation); `.git/` is always ignored.
- File-mode tracking is limited on Windows; files default to `100644`.
- No CRLF translation: bytes are stored verbatim so hashes stay stable.
- Real network pushes are not exercised in CI; the smart-HTTP protocol is tested
  against an in-process `httptest` server.
