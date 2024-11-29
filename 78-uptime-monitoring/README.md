# Uptime Monitoring Tool

A self-hosted uptime monitor (in the spirit of Uptime Kuma / Pingdom /
UptimeRobot) built for [John Crickett's Coding Challenge #78](https://codingchallenges.substack.com/p/coding-challenge-78-uptime-monitoring).

It probes a list of HTTP(S) endpoints and raw TCP hosts on independent
per-monitor schedules, records whether each target is **up** or **down** plus how
long the check took, persists the results as time-series data, computes uptime
percentages and historical response-time graphs, and fires **debounced** alerts
(log / webhook / email) on confirmed state transitions. A small embedded web
dashboard and a REST/JSON API expose live status, history, and SLIs.

## Highlights

- **HTTP HEAD/GET checks** — HEAD by default; GET captures **TTFB**
  (time-to-first-byte) via `net/http/httptrace` plus full round-trip latency.
- **TCP checks** for non-HTTP services (`host:port`).
- **Per-monitor schedules** driven by an **injectable clock**, so the scheduler
  is tested with simulated time — no real sleeping.
- **Bounded worker pool + per-check timeouts** — a slow/hung target never delays
  or starves other monitors; overrun ticks are skipped, not queued.
- **Durable time-series store** — pure-Go SQLite by default (no cgo), with an
  in-memory store for tests; results survive restart.
- **Anti-flap alerting** — a DOWN alert fires only after *N* consecutive
  failures and only on the actual UP→DOWN transition; recovery fires exactly
  once. Restart seeds last-known status so the first post-restart probe doesn't
  false-alert.
- **REST API + embedded dashboard + CSV/JSON export.**
- **Graceful shutdown** on SIGINT/SIGTERM: in-flight checks drain, store flushes.

## Layout

```
cmd/uptime/main.go            wiring, flags, signals, graceful shutdown
internal/
  monitor/   domain types (Monitor, CheckResult, Status, Kind, StatusRange)
  clock/     Clock interface, RealClock, FakeClock (deterministic test clock)
  checker/   Checker interface, HTTPChecker (HEAD/GET+TTFB), TCPChecker
  scheduler/ per-monitor tick loops, bounded pool, ctx cancellation, metrics
  store/     Store interface; SQLite (durable) + Memory (tests)
  aggregate/ pure SLI math: uptime %, history bucketing, latency percentiles
  alert/     StateTracker (anti-flap) + Log/Webhook/Email/Multi alerters
  api/        REST handlers + embedded single-page dashboard
```

## Build & run

```bash
go mod tidy
go build -o uptime ./cmd/uptime
./uptime -config config.yaml      # scheduler + API/dashboard on :8080
```

Then open <http://localhost:8080> for the dashboard.

The bundled [`config.yaml`](./config.yaml) is a working example. To run with no
on-disk database, set `storage.driver: memory`.

## Configuration

```yaml
server:
  listen: ":8080"
storage:
  driver: sqlite          # sqlite | memory
  dsn: ./uptime.db
  retention: 720h         # informational (raw-row pruning is an extension)
defaults:                 # inherited by monitors that omit a field
  interval: 60s
  timeout: 10s            # must be < interval
  method: HEAD
  expect_status: [200-399]
  latency_threshold: 0    # 0 = disabled; otherwise breach => Down
  alert_threshold: 3      # consecutive failures before a DOWN alert
alerters:
  - type: log
  - type: webhook
    url: https://hooks.example.com/uptime
  - type: email
    smtp: smtp.example.com:587
    from: alerts@example.com
    to: [oncall@example.com]
monitors:
  - id: api-home
    name: "API Home"
    kind: http            # http | tcp
    target: https://api.example.com/health
    interval: 30s
    timeout: 5s
    method: GET           # capture TTFB
    expect_status: [200]
    latency_threshold: 800ms
    alert_threshold: 2
  - id: db-tcp
    name: "Primary DB"
    kind: tcp
    target: db.internal:5432
    interval: 15s
```

Validation at load rejects: duplicate/empty `id`, invalid `kind`, unparseable
`target`, `timeout >= interval`, out-of-range status codes, and unknown keys.

## REST API

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/monitors` | list monitors + live status |
| POST | `/api/monitors` | add a monitor at runtime (JSON body) |
| DELETE | `/api/monitors/{id}` | remove a monitor and its results |
| GET | `/api/monitors/{id}/history?from=&to=&bucket=` | bucketed time-series for graphs |
| GET | `/api/monitors/{id}/uptime?from=&to=` | uptime % over a range (`null` for an empty window) |
| GET | `/api/export?format=csv\|json&from=&to=` | data export |
| GET | `/healthz` | liveness |
| GET | `/metrics` | Prometheus-style counters |
| GET | `/` | dashboard |

`from`/`to` accept RFC3339 timestamps or unix-millis; both default to the last
24 hours. `bucket` accepts a Go duration (e.g. `1h`, `5m`).

Example:

```bash
curl localhost:8080/api/monitors
curl "localhost:8080/api/monitors/api-home/uptime?from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z"
curl "localhost:8080/api/export?format=csv" -o export.csv
```

## Tests

The entire suite runs **offline** with no real sleeping:

- **Checker** against an in-process `httptest.Server` (200→up, 500/timeout→down,
  TTFB on GET, connection-refused, latency-threshold) and a local `net.Listen`
  TCP listener (open→up, closed→down).
- **Scheduler** with the `FakeClock`: deterministic interval counting,
  independent intervals, overrun-tick skipping, slow-monitor isolation, and
  graceful drain on cancel.
- **Aggregate** uptime %, bucketing, and percentiles (table-driven, pure).
- **Alert** transitions: fires once at threshold, debounces, recovers once,
  anti-flap below threshold, no spurious first-Up, seed-on-restart; webhook
  against `httptest`, email against an injected send seam.
- **Store** round-trips for both Memory and SQLite (incl. persistence across
  reopen).
- **API** handlers via `httptest.NewRecorder` over a Memory store.

```bash
go test ./...
```

> **Note on `-race`:** the race detector requires a 64-bit C toolchain (cgo).
> On a machine with one installed, run `go test ./... -race`. The tests
> themselves are race-free and use no wall-clock sleeps in their success paths.

## Notes / caveats

- The SQLite store uses the pure-Go `modernc.org/sqlite` driver, so the binary
  builds and runs without cgo or a C compiler.
- `POST /api/monitors` persists to the store; the running scheduler picks up
  newly added monitors on restart (live re-scheduling is a possible extension).
- `storage.retention` is parsed and surfaced but raw-row pruning/downsampling is
  left as an extension.
