# loadtester — HTTP(S) Load Tester

Coding Challenge #41/#42 — *Build your own HTTP(S) load tester*.

A small command-line load-testing tool in the spirit of `ab`, `hey`, and `wrk`.
It fires a configurable number of HTTP requests at a target URL (or a cycled
file of URLs) using a fixed-size pool of concurrent workers, captures
per-request timing (total time, time-to-first-byte, time-to-last-byte),
classifies each response, and prints a statistical summary.

Standard library only — no third-party dependencies.

## Build

```sh
go build -o loadtester ./cmd/loadtester
```

Or run directly:

```sh
go run ./cmd/loadtester -u https://example.com -n 200 -c 10
```

## Usage

```
loadtester -u <url> [options]
loadtester -f <file> [options]

  -u  string   Target URL (required unless -f given). e.g. https://example.com
  -f  string   File of URLs (one per line; '#' comments and blank lines ignored).
               Cycled if it holds fewer URLs than -n. Mutually exclusive with -u.
  -n  int      Total number of requests to send.            (default 10)
  -c  int      Number of concurrent workers.                (default 1)
  -m  string   HTTP method: GET|POST|PUT|PATCH|DELETE|HEAD. (default GET)
  -H  string   Custom header 'Key: Value'. Repeatable.      (e.g. -H "Authorization: Bearer x")
  -d  string   Request body data (literal string, or @file to read from a file).
  -t  duration Per-request timeout, e.g. 5s, 500ms.         (default 30s)
  -json        Emit the summary as JSON instead of a table. (default false)
```

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Ran to completion (at least one success). |
| 1 | Invalid configuration. |
| 2 | All requests failed (useful for CI gating). |

### Examples

```sh
# 1000 requests, 50 concurrent, 5s timeout
loadtester -u https://example.com -n 1000 -c 50 -t 5s

# POST with a JSON body from a file and an auth header
loadtester -u https://api.example.com/v1/things -m POST \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d @payload.json -n 500 -c 20

# Cycle through a list of URLs
loadtester -f urls.txt -n 300 -c 16

# Machine-readable output for CI
loadtester -u https://example.com -n 200 -c 10 -json
```

### Example output

```
Summary:
  Total requests:     100
  Success (2XX):      100
  Failed:               0
  Wall clock:         48.2ms
  Throughput:        2073.2 req/s

Status codes:
  200:     100

Latencies (ms):               min     mean      p50      p90      p95      p99      max
  Total:                   3.05     3.66     3.56     3.81     5.53     5.53     6.05
  Time to first byte:      3.05     3.66     3.56     3.81     5.53     5.53     6.05
  Time to last byte:       3.05     3.66     3.56     3.81     5.53     5.53     6.05
```

## How it works

```
flags -> config.Config            validated, immutable configuration
      -> urls.Provider            produces exactly N targets (single, or file cycled)
      -> runner.Run               scheduler goroutine -> jobs chan -> C workers
                                  -> Requester.Do (httptrace timing) -> results chan
      -> stats.Collector          single-goroutine drain, no locking
      -> Summarize (pure)         counts, histogram, throughput, latency percentiles
      -> report (table | JSON)    stdout
```

- **Worker pool.** A scheduler goroutine pushes `N` jobs onto a buffered channel
  and closes it; `C` worker goroutines range over the channel and send results to
  a results channel. A `sync.WaitGroup` closer goroutine closes the results
  channel once all workers return, so the collector loop terminates cleanly with
  no goroutine leaks.
- **Timing.** The production `HTTPRequester` attaches an `httptrace.ClientTrace`
  to capture TTFB (`GotFirstResponseByte`) and always drains the body to EOF so
  TTLB and connection reuse are correct.
- **Timeouts** are enforced per request via `context.WithTimeout` and classified
  into a distinct `timeout` bucket, separate from other transport errors.
- **Ctrl-C / SIGTERM** cancels the root context; in-flight requests finish and a
  partial summary is still printed.
- **Stats** are pure functions over recorded samples. Percentiles use the
  nearest-rank method (`index = ceil(p/100 * N) - 1`, clamped). Only
  successfully-timed requests feed latency percentiles; transport errors and
  timeouts contribute to counts only. Success is strictly 2XX.

## Project layout

```
41-load-tester/
  go.mod
  cmd/loadtester/main.go        wiring: flags -> run -> report
  internal/config/              flag parsing + validation
  internal/urls/                URL provider (single + file cycling)
  internal/runner/              worker pool, scheduler, Requester iface + HTTP impl
  internal/stats/               Sample, Collector, Summarize, percentile (pure)
  internal/report/              table + JSON reporters
```

## Tests

All tests are deterministic and run with **no external network** — integration
tests use in-process `net/http/httptest` servers.

```sh
go test ./...
```

Coverage includes: percentile/mean/min/max math against hand-computed values;
`Summarize` counts, status histogram and throughput; URL file parsing and
cycling; config validation (missing source, bad method/scheme/header, `-c`
clamping, `-d @file`); the worker pool against a fake `Requester` (asserts exactly
`N` calls, peak concurrency never exceeds `C`, every job processed once); and
end-to-end runs against `httptest` servers for 200 success, server delay, 500
failures, per-request timeouts, connection-refused errors, and header/body
propagation.

Race detector:

```sh
go test -race ./...
```

> Note: `-race` requires cgo with a 64-bit C toolchain. On a machine without one
> (e.g. only a 32-bit MinGW gcc) the race build fails with
> `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`; this is a
> toolchain limitation, not a test failure. The non-race suite passes fully.

## Notes & limitations

- Memory is O(N) in the number of requests (one sample retained per request).
  A fixed-bucket histogram would bound memory for very large `N`; nearest-rank on
  a sorted slice was chosen here for exactness at the modest scale of this
  challenge.
- TLS verification uses Go defaults. Connection pooling is sized to `2 * C`.
