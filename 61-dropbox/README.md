# Dropbox file sync (Coding Challenge #61)

A file-synchronisation system with two cooperating Go programs:

- **`dropd`** — a multi-user server that stores files and exposes a small REST
  API. It is the single source of truth.
- **`dropbox`** — a client that watches a local folder and keeps it in two-way
  sync with the server.

This README is the operator's guide.

## Highlights

- **Three-way diff** (local index vs. remote index vs. last-synced *baseline*) so
  a *new remote file* is never confused with a *locally deleted* one.
- **SHA-256 content hashing** is the authority for change detection, with
  `(size, mtime)` as a fast pre-filter so unchanged files are not re-hashed.
- **Bidirectional sync**: uploads, downloads and deletes propagate both ways.
- **Conflict handling that never loses data**: if a path changed on both sides,
  the server copy wins the canonical path and the local edit is preserved as a
  `name (conflicted copy YYYY-MM-DD HH-MM-SS).ext` sidecar (also uploaded).
- **Atomic, versioned writes** everywhere: every file and index is written to a
  temp file and `rename`d into place; server `PUT` uses optimistic concurrency
  (`If-Match` version) and verifies the declared SHA-256 before committing.
- **Nested folders**, arbitrary depth, paths normalised to `/` on the wire.
- **Pure sync engine**: `sync.Plan(local, remote, base, now) []Action` does no
  I/O, so it is exhaustively table-tested.

## Build

```
go build ./...                 # both binaries compile
go build -o dropd  ./cmd/dropd
go build -o dropbox ./cmd/dropbox
```

No third-party dependencies — standard library only.

## Quick start

```sh
# 1. create a user on the server (password piped on stdin); prints a token
printf 'secretpw' | dropd --config dropd.json useradd alice

# 2. run the server
dropd --addr 127.0.0.1:8080 --data ./data --config dropd.json serve

# 3. set up a client folder and log in
dropbox init  --root ./drive --server http://127.0.0.1:8080
printf 'secretpw' | dropbox login --root ./drive --server http://127.0.0.1:8080 \
    --user alice --password-stdin

# 4. sync
echo hello > ./drive/a.txt
dropbox sync  --root ./drive            # one-shot push/pull, then exit
dropbox status --root ./drive           # preview the plan, change nothing
dropbox watch --root ./drive --interval 10s   # daemon: poll + sync
```

## CLI reference

### `dropbox` (client)

```
dropbox <command> [flags]

Global flags:
  --config <path>   config file (default <root>/.dropbox/config.json)
  --server <url>    server base URL            (env DROPBOX_SERVER)
  --token  <tok>    bearer token              (env DROPBOX_TOKEN)
  --root   <dir>    local sync root folder
  -v, --verbose     verbose logging
  --dry-run         print the plan; perform no transfers

Commands:
  init    create <root>/.dropbox and write config
  login   exchange credentials for a bearer token (--password-stdin)
  status  show the plan without acting
  sync    one diff + transfer pass, then exit
  watch   daemon: poll ticker + sync on change (--interval, default 15s)

Exit codes: 0 ok/converged · 1 runtime error · 2 conflicts written · 3 auth failure
```

### `dropd` (server)

```
dropd [flags] [serve|useradd <user>]
  --addr   <host:port>   listen address (default :8080)
  --data   <dir>         storage root  (default ./data)
  --config <path>        users/tokens config (default ./dropd.json)
  --tls-cert / --tls-key optional TLS

  dropd useradd <user>   create/update a user; password piped on stdin
  dropd serve            run the HTTP server (default)
```

## REST API (v1)

| Method & path            | Purpose                              | Notes |
| ------------------------ | ------------------------------------ | ----- |
| `POST /v1/auth/login`    | exchange credentials for a token     | `{user,password}` → `{token}` |
| `GET  /v1/files`         | list remote index                    | `{files:[{path,hash,size,version}]}` |
| `GET  /v1/files/{path…}` | download file content                | streams; `X-Content-SHA256`, `X-Version` headers |
| `PUT  /v1/files/{path…}` | upload/replace file                  | `If-Match: <version>`, `X-Content-SHA256` verified |
| `DELETE /v1/files/{path…}` | delete file                        | `If-Match: <version>` |

All non-login calls require `Authorization: Bearer <token>` and are scoped to
that user. Bad/missing token → 401; version mismatch → 409; SHA-256 mismatch →
422; path traversal / absolute paths → 400.

## How sync decides (three-way diff)

For each path across {local, remote, baseline}, "changed" means the hash differs
from the baseline:

| local        | remote          | action                         |
| ------------ | --------------- | ------------------------------ |
| unchanged    | unchanged       | no-op                          |
| changed/new  | unchanged/absent| upload                         |
| unchanged/absent | changed/new | download                       |
| deleted      | unchanged       | delete remote                  |
| unchanged    | deleted         | delete local                   |
| changed      | changed, same hash | no-op (converged)           |
| changed      | changed, diff hash | **conflict** (sidecar + keep server) |
| deleted      | changed         | download (restore — no data loss) |
| changed      | deleted         | upload (resurrect — no data loss) |

The baseline is the on-disk index at `<root>/.dropbox/index.json`; after a pass
it is advanced **only for paths that are byte-identical on both sides**, so an
interrupted transfer is simply retried on the next run.

## Testing

```
go test ./...            # all tiers
go test ./... -race      # requires a working cgo/64-bit C toolchain
```

Tiers (all hermetic, `t.TempDir()`, injected clock — no real network):

- `internal/hash` — known SHA-256 vectors; streamed == one-shot.
- `internal/index` — scan/ignore/nesting; fast-path proven by counting hash
  calls (0 re-hashes when unchanged, 1 after an mtime change); save/load round-trip.
- `internal/sync` — exhaustive table tests for every diff row plus a fixed-point
  property (apply a plan, re-plan, expect empty → convergence).
- `internal/server` — versioned atomic writes, concurrent same-path PUTs (exactly
  one winner), SHA-256 mismatch rejection, path-traversal rejection, per-user scoping.
- `internal/client` — end-to-end against an in-process `httptest` server:
  upload/download/modify/delete/rename propagation, conflict (sidecar + canonical
  = server copy), two-client byte-identical convergence, and resumability after a
  failed transfer.

## Known limitations

- **Empty directories** are not first-class in a path-keyed store (a directory
  exists only because it contains files). Use a `.keep` sentinel if needed.
- **Symlinks** are not followed (avoids cycles / escaping the root); they are skipped.
- **Case-insensitive filesystems**: paths are compared case-sensitively on the
  wire, so two names differing only in case can collide on such a volume.
- **`-race`** needs a 64-bit C toolchain (cgo); on machines without one the suite
  still runs fully without the flag. The store's concurrency test passes either way.
- Passwords are stored as salted-free SHA-256 for a dependency-free build; bcrypt
  is the documented production upgrade path.
