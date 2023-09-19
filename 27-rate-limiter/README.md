# Rate Limiter — Coding Challenge #27

A production-quality HTTP API rate limiter in Go. It caps how many requests a
single client (keyed by IP) may make in a time period and rejects the excess
with **HTTP 429 Too Many Requests**. Four classic algorithms sit behind one
`Limiter` interface, state lives in a pluggable backend (in-memory or Redis),
and all time is read through an injectable clock so the whole test suite is
deterministic and never sleeps.

## Algorithms

| Algorithm (`-algo`) | Idea | Strength / weakness |
|---|---|---|
| `token-bucket` | Tokens refill at a fixed rate up to a capacity; each request spends one. | Allows bursts up to capacity; bounds sustained rate. |
| `fixed-window` | Count requests in wall-clock-aligned fixed windows. | Cheapest; can admit up to 2× the limit across a window boundary. |
| `sliding-log` | Keep a timestamp per request, drop those older than the window. | Exact, no boundary artifact; memory grows with traffic. |
| `sliding-counter` | Weighted blend of current + previous fixed-window counts. | Two ints/key, smooths the boundary spike, distribution-friendly. |

## Layout

```
cmd/ratelimiter/main.go        CLI flags, wiring, graceful shutdown
internal/clock/                Clock, RealClock, FakeClock (injectable time)
internal/store/                Store interface, State, MemoryStore (+janitor), RedisStore
internal/limiter/              Limiter interface, Decision, the four algorithms, factory
internal/httpmw/               RateLimit middleware + client-IP key extraction
```

The HTTP layer depends only on the `Limiter` interface; algorithms depend only
on the `Store` interface. Swapping algorithm or backend touches neither the
middleware nor the algorithm math.

## Build & test

```sh
go build ./...
go test ./...              # fast, fully offline (Redis paths use miniredis)
go test ./... -race        # if a 64-bit C toolchain is available (see caveats)
```

The Redis-backed tests run against an in-process **miniredis** by default, so
the suite is green offline with no server required. To exercise a real server,
set `REDIS_ADDR`:

```sh
REDIS_ADDR=127.0.0.1:6379 go test ./internal/store/ ./internal/limiter/
```

## Run

```sh
go run ./cmd/ratelimiter -algo token-bucket -limit 10 -refill 1
```

- `GET /unlimited` → `200`, body `Unlimited! Let's Go!` (never limited).
- `GET /limited` → `200`, body `Limited, don't over use me!` while under the
  limit; otherwise `429`, body `Too Many Requests`, with `Retry-After` and
  `X-RateLimit-Limit` / `-Remaining` / `-Reset` headers.

### CLI flags

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `127.0.0.1:8080` | Listen address. |
| `-algo` | `token-bucket` | `token-bucket` \| `fixed-window` \| `sliding-log` \| `sliding-counter`. |
| `-limit` | `10` | Token-bucket capacity, or max requests per window. |
| `-window` | `1s` | Refill reference / fixed / sliding window size. |
| `-refill` | `1` | Tokens per second (token bucket only). |
| `-backend` | `memory` | `memory` or `redis`. |
| `-redis-addr` | `127.0.0.1:6379` | Redis address when `-backend=redis`. |
| `-key-header` | `""` | Trusted proxy header for client IP (else `RemoteAddr`). |
| `-fail-open` | `true` | On backend error, allow (`true`) or deny with 503 (`false`). |
| `-idle-ttl` | `10m` | Evict in-memory keys idle longer than this. |

### Manual check

```sh
curl -s http://127.0.0.1:8080/unlimited
for i in $(seq 1 12); do curl -s -o /dev/null -w "%{http_code} " http://127.0.0.1:8080/limited; done; echo
```

The first 10 print `200`, then `429` once the bucket drains.

## Distributed mode (Step 6)

With `-backend=redis`, state lives in Redis and the limit is enforced
**globally** across instances. Run two servers sharing one Redis:

```sh
go run ./cmd/ratelimiter -backend=redis -addr 127.0.0.1:8080 -algo sliding-counter -limit 60 -window 1m &
go run ./cmd/ratelimiter -backend=redis -addr 127.0.0.1:8081 -algo sliding-counter -limit 60 -window 1m &

for i in $(seq 1 60); do curl -s -o /dev/null http://127.0.0.1:8080/limited; done   # spend the budget on A
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8081/limited              # B now returns 429
```

Atomicity across instances is achieved with optimistic concurrency: each step
runs as `WATCH` + `MULTI`/`EXEC` over the JSON-encoded state and retries on the
rare conflict, so concurrent writers never lose updates. (A per-algorithm Lua
script is the lower-latency alternative; the optimistic approach was chosen here
because it keeps a single Go implementation of each algorithm running unchanged
against both backends.) Keys carry a TTL so abandoned state self-expires.

## Design notes

- **Injectable clock** — every algorithm reads time via `clock.Clock`; tests
  inject a `FakeClock` and call `Advance`, so window rolls, refills, and
  boundary cases are verified without real time passing.
- **Concurrency** — the read-modify-write per key is atomic (mutex in memory,
  optimistic transaction in Redis); the concurrency tests fire hundreds of
  goroutines at one key and assert the admitted count equals the limit exactly.
- **Memory bounds** — the in-memory store runs a janitor that evicts idle keys.
- **Proxy spoofing** — `X-Forwarded-For` / `X-Real-IP` is trusted only when you
  pass `-key-header`; otherwise the un-spoofable `RemoteAddr` is used.
- **Backend failure** — a limiter error degrades per `-fail-open` (default:
  serve) and never panics on the request path.

## Caveats

- `go test -race` requires cgo with a 64-bit C toolchain. The provided Windows
  toolchain here is 32-bit only (`gcc: sorry, unimplemented: 64-bit mode not
  compiled in`), so the race detector cannot run in this environment; the suite
  passes without it and the concurrency tests still assert exact admit counts.
  On a host with a 64-bit gcc/clang, `go test ./... -race` runs as intended.
- The real-Redis integration path is opt-in via `REDIS_ADDR`; by default Redis
  logic is exercised against in-process miniredis.
