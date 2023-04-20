# lb — HTTP Load Balancer

A small, production-shaped Layer 7 (HTTP) **reverse proxy / load balancer** in Go.
It listens on a configured port, forwards each request to one of N backends chosen
by a **round-robin** scheduler, and runs a **background health checker** that drops
unhealthy backends from rotation and re-adds them automatically on recovery. It
serves many clients concurrently and shuts down gracefully.

This implements [John Crickett's Coding Challenge #5](https://codingchallenges.substack.com/p/coding-challenge-5);

## Features

- **Concurrent reverse proxy** built on `net/http/httputil.ReverseProxy` — correct
  header/hop-by-hop handling and streaming bodies for free.
- **Round-robin scheduling** via a lock-free atomic counter, computed over the live
  healthy snapshot so dropped backends are simply absent.
- **Pluggable `Scheduler` interface** — drop in least-connections / weighted / IP-hash
  later without touching the proxy.
- **Background health checks** — one ticker goroutine fans out one probe per backend
  each round; a non-200 / timeout / connection failure marks a backend down, a 200
  re-adds it. State changes are the only thing logged or locked.
- **Idiomatic concurrency** — `sync/atomic` for the health flag, counter, and conn
  count; `sync.RWMutex` only guards structural pool changes; one goroutine per request.
- **Config via CLI flags + optional YAML file**, flags override file values.
- **Graceful shutdown** on SIGINT/SIGTERM with a bounded drain.
- **Structured logging** via `log/slog`.

## Layout

```
cmd/lb/main.go              wiring, signal handling, graceful shutdown
internal/backend/           Backend: per-upstream reverse proxy + atomic state
internal/pool/              BackendPool: healthy-snapshot under RWMutex
internal/scheduler/         Scheduler interface + RoundRobin
internal/health/            background HealthChecker (ticker + fan-out probes)
internal/proxy/             LB http.Handler, ErrorHandler, upstream transport
internal/config/            flags + YAML loading, validation
lb.yaml                     sample config
```

## Build

```bash
go build ./...
# or a single static binary:
go build -o lb ./cmd/lb       # lb.exe on Windows
```

Requires Go 1.24+. The only dependency is `gopkg.in/yaml.v3`.

## Test

```bash
go test ./...                 # full unit + integration suite
go test -race ./...           # race detector (needs a working cgo/64-bit C toolchain)
```

The suite uses `httptest` backends to verify:
- exact round-robin distribution (3 backends, 6 requests → A,B,C,A,B,C),
- the health checker dropping a backend on non-200 and re-adding it on recovery,
- `503` when all backends are down and `502` when a backend dies mid-request,
- large-body streaming, and concurrent load while backends flap up/down.

## Run

```bash
# Step 1 — single backend
lb -listen :8080 -backend http://localhost:8081

# Step 2 — round-robin over three backends
lb -listen :8080 \
   -backend http://localhost:8081 \
   -backend http://localhost:8082 \
   -backend http://localhost:8083

# Step 3 — with health checks
lb -listen :8080 \
   -backend http://localhost:8081 -backend http://localhost:8082 \
   -health-path /health -health-interval 10s -health-timeout 2s

# From a config file
lb -config ./lb.yaml
```

### CLI flags

```
-listen string          Address:port to listen on (default ":8080")
-backend value          Backend URL; repeatable
-config string          Path to YAML config file (optional)
-health-path string     Health check request path (default "/health")
-health-interval dur    Interval between health check rounds (default 10s)
-health-timeout dur     Per-probe timeout (default 2s)
-strategy string        Scheduling strategy: round_robin (default "round_robin")
-log-level string       debug|info|warn|error (default "info")
```

Precedence is **defaults < config file < explicitly-set flags**. Providing
`-backend` on the CLI replaces the file's backend list entirely.

### Sample config (`lb.yaml`)

```yaml
listen: ":8080"
backends:
  - http://localhost:8081
  - http://localhost:8082
  - http://localhost:8083
health:
  path: "/health"
  interval: "10s"
  timeout: "2s"
strategy: "round_robin"
log_level: "info"
proxy:
  dial_timeout: "5s"
  response_header_timeout: "30s"
  shutdown_timeout: "15s"
```

## Manual smoke test

```bash
# three identifiable backends
python3 -m http.server 8081 &
python3 -m http.server 8082 &
python3 -m http.server 8083 &

lb -listen :8080 \
   -backend http://localhost:8081 \
   -backend http://localhost:8082 \
   -backend http://localhost:8083

# watch the rotation in lb's logs
for i in $(seq 1 6); do curl -s localhost:8080 >/dev/null; done
```

Kill one backend → within one health interval the LB stops routing to it and keeps
serving from the rest; restart it → it rejoins rotation within one interval.

## Behavior notes

| Situation | Response |
|---|---|
| All backends unhealthy | `503 Service Unavailable`; nothing is dialed |
| Backend dies mid-request | `ErrorHandler` marks it down, returns `502 Bad Gateway` |
| Health path returns non-200 / times out / refuses | backend marked unhealthy |
| Recovered backend returns 200 | re-added within one health interval |
| Empty/duplicate/malformed backends, bad durations | startup error, non-zero exit |
| SIGINT / SIGTERM | graceful drain bounded by `shutdown_timeout` |
