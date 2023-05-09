# redis-server — Coding Challenge #8

A lite Redis clone in Go: a concurrent TCP server speaking **RESP2**, backed by
an in-memory, TTL-aware key/value store. It is wire-compatible with the real
`redis-cli` and survives `redis-benchmark`.

## Features

- **RESP2 codec** — serializer/deserializer for all five wire types (simple
  string, error, integer, bulk string, array) plus the null/empty forms
  (`$-1`, `*-1`, `$0`, `*0`). Bulk strings are read by length, so payloads
  containing `\r`, `\n`, or NUL survive intact. Inline commands (raw telnet) are
  also accepted.
- **Concurrent server** — one goroutine per connection; the shared store is the
  only cross-goroutine state and is guarded by a `sync.RWMutex`. A panic in any
  handler is recovered per-connection, so a single bad request never crashes the
  server.
- **Commands** — `PING`, `ECHO`, `SET`, `GET`, `EXISTS`, `DEL`, `INCR`, `DECR`,
  `LPUSH`, `RPUSH`, `LRANGE`, `SAVE`, `COMMAND`. Command names are
  case-insensitive; keys and values are binary-safe.
- **SET options** — `EX` / `PX` / `EXAT` / `PXAT`, `NX` / `XX`, `KEEPTTL`,
  `GET`, with Redis-compatible validation and error messages.
- **Expiry** — lazy eviction on every read path, plus an optional background
  sweeper for active eviction (100 ms tick; disable with `--no-active-expiry`).
- **Persistence** — `SAVE` writes a JSON snapshot atomically (temp file +
  rename); the snapshot is loaded on startup if present.

## Layout

```
cmd/redis-server/main.go     flag parsing, wiring, graceful shutdown
internal/resp/               RESP2 Value type, reader, writer (+ tests)
internal/store/              concurrent TTL store, persistence (+ tests)
internal/server/             TCP accept loop, command dispatch, handlers (+ tests)
```

## Build

```bash
go build ./...
go build -o redis-server ./cmd/redis-server
```

## Run

```bash
go run ./cmd/redis-server --port 6379
```

Flags:

| Flag                 | Default       | Description                                   |
|----------------------|---------------|-----------------------------------------------|
| `--port`             | `6379`        | Port to listen on                             |
| `--host`             | `127.0.0.1`   | Bind address (`0.0.0.0` for all interfaces)   |
| `--dbfile`           | `dump.rdb`    | Snapshot file path for `SAVE` / load          |
| `--no-active-expiry` | `false`       | Disable the background expiry sweeper         |

## Test

```bash
go test ./...              # unit + integration tests
go test -race ./...        # with the race detector (needs a cgo/gcc toolchain)
go vet ./...
gofmt -l .
```

The suite covers RESP round-trip and edge cases (null/empty forms, binary-safe
payloads, malformed input), store semantics (set/get/del/exists, TTL with an
injectable clock, INCR/DECR overflow, list ops, WRONGTYPE), persistence
round-trips, and concurrency (N goroutines incrementing a shared key, both at
the store level and over real sockets).

> Note: `go test -race` requires cgo with a working 64-bit C toolchain. If one
> is not installed, run `go test ./...` (the concurrency tests still execute and
> assert correctness; only the race instrumentation is skipped).

## Compatibility with real `redis-cli`

The server speaks standard RESP2, so the official client works with no extra
flags:

```bash
redis-cli -p 6379 PING                 # PONG
redis-cli -p 6379 ECHO "hello world"   # "hello world"
redis-cli -p 6379 SET name claude      # OK
redis-cli -p 6379 GET name             # "claude"
redis-cli -p 6379 SET tmp x PX 100     # OK
sleep 0.2; redis-cli -p 6379 GET tmp   # (nil)
redis-cli -p 6379 EXISTS name missing  # (integer) 1
redis-cli -p 6379 DEL name             # (integer) 1
redis-cli -p 6379 INCR counter         # (integer) 1
redis-cli -p 6379 RPUSH mylist a b c   # (integer) 3
redis-cli -p 6379 LRANGE mylist 0 -1   # 1) "a" 2) "b" 3) "c"
redis-cli -p 6379 SAVE                 # OK
redis-cli -p 6379 NOPE                 # (error) ERR unknown command 'NOPE'
```

Benchmark it like real Redis:

```bash
redis-benchmark -t set,get -n 100000 -q
```
