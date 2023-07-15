# Memcached Server (Coding Challenge #17)

A from-scratch, production-ready in-memory cache server that speaks the
**memcached classic text protocol** over TCP. Written in Go with zero
third-party dependencies. Interoperable with `telnet`/`nc` and real memcached
client libraries (`pymemcache`, `bradfitz/gomemcache`).

## Build & run

```sh
cd 17-memcached-server
go build -o bin/memcached ./cmd/memcached
./bin/memcached                 # 127.0.0.1:11211, 64 MiB budget
./bin/memcached -p 12345        # custom port
./bin/memcached -m 0 -v         # unlimited memory, verbose logging
```

### CLI flags

| Flag | Meaning | Default |
|------|---------|---------|
| `-p <port>` | TCP port to listen on | `11211` |
| `-l <addr>` | Interface/host to bind | `127.0.0.1` |
| `-m <mb>`   | Max item memory in MiB (`0` = unlimited); triggers LRU eviction | `64` |
| `-c <n>`    | Soft max concurrent connections | `1024` |
| `-v`        | Verbose per-command logging | off |

Graceful shutdown on `SIGINT`/`SIGTERM`: the listener stops accepting and
in-flight connections drain.

## Supported commands

- **Storage:** `set`, `add`, `replace`, `append`, `prepend`, `cas`
  (each with `<flags> <exptime> <bytes> [noreply]`; `cas` adds `<cas_unique>`).
- **Retrieval:** `get`, `gets` (one or more keys; `gets` returns the CAS token).
- **Deletion:** `delete <key> [noreply]`.

Responses: `STORED`, `NOT_STORED`, `EXISTS`, `NOT_FOUND`, `DELETED`, `END`,
`ERROR`, `CLIENT_ERROR <msg>`, `SERVER_ERROR <msg>`.

### TTL (`exptime`) rules

- `0` — never expires.
- `1..2592000` (≤ 30 days) — relative seconds from now.
- `> 2592000` — absolute Unix timestamp.
- `< 0` — expires immediately.

Expiry is **lazy** (checked on access); a 1 s background janitor also reaps
expired items to reclaim memory, but correctness never depends on it.

### Memory & LRU

Values are bounded per-object at 1 MiB (`SERVER_ERROR object too large for
cache`). Total item memory is bounded by `-m`; inserting past the budget evicts
the **least-recently-used** items first. Recency is updated on every successful
`get`/`gets` and store.

## telnet / real-client compatibility

Drive it directly with `telnet` (type the lines; the data block follows the
storage command line):

```
$ telnet 127.0.0.1 11211
set foo 0 0 3
bar
STORED
get foo
VALUE foo 0 3
bar
END
gets foo
VALUE foo 0 3 1
bar
END
append foo 0 0 3
baz
STORED
get foo
VALUE foo 0 6
barbaz
END
delete foo
DELETED
```

It is wire-compatible with standard client libraries, e.g. `pymemcache`:

```python
from pymemcache.client.base import Client
c = Client(("127.0.0.1", 11211))
c.set("foo", "bar")
assert c.get("foo") == b"bar"
```

The same script can be pointed at a real `memcached` to confirm identical
responses (differential testing).

## Layout

```
17-memcached-server/
├── cmd/memcached/main.go          # flags, wiring, signal handling
└── internal/
    ├── protocol/                  # command parsing + response formatting
    ├── store/                     # Item/Store: TTL, CAS, LRU, accounting
    └── server/                    # TCP listener, connection lifecycle, dispatch
```

## Tests

```sh
go vet ./...
go test ./...
go test -race ./...
```

Coverage includes protocol parse/round-trip, every command, TTL expiry with an
injectable clock, CAS conflict resolution, LRU eviction order, memory
accounting, store concurrency, and socket-level integration tests (basic flow,
multi-key get, CAS, errors, `noreply`, expiry, pipelining, concurrent clients).
```
