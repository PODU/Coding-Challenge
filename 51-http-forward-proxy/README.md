# Coding Challenge #51 — HTTP Forward Proxy

A production-ready forward proxy for HTTP and HTTPS, written in Go. It parses
absolute-form proxy requests, forwards them to the origin while stripping
hop-by-hop headers and injecting `X-Forwarded-For`, blocks forbidden hosts,
filters response bodies for banned words, logs all access to a file and stdout,
and tunnels HTTPS via the `CONNECT` method.

## Features

| Step | Feature | Where |
|------|---------|-------|
| 1 | Basic forwarding: absolute-form parsing, hop-by-hop stripping, `X-Forwarded-For` | `internal/proxy/proxy.go` (`forward`) |
| 2 | Host blocklist → `403 Forbidden` | `internal/filter` + `forbidden-hosts.txt` |
| 3 | Banned-word response-body filter → `403 Forbidden` | `internal/filter` + `banned-words.txt` |
| 4 | Access logging (client IP, URL, status) to file **and** stdout | `internal/logx` |
| 5 | HTTPS `CONNECT` tunneling (bidirectional byte copy) | `internal/proxy/proxy.go` (`tunnel`) |

## Build & Run

Requires Go 1.24+.

```sh
# run with defaults (listens on :8989)
go run .

# custom port + filter files
go run . -addr :9090 -forbidden-hosts hosts.txt -banned-words words.txt

# build a static binary
go build -o http-forward-proxy .
```

### Smoke tests (require network access)

```sh
go run . &
curl --proxy "http://localhost:8989" "http://httpbin.org/ip"     # forward (HTTP)
curl --proxy "http://localhost:8989" "https://httpbin.org/ip"    # CONNECT (HTTPS)
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:8989` | Listen address |
| `-forbidden-hosts` | `forbidden-hosts.txt` | Path to forbidden hosts file |
| `-banned-words` | `banned-words.txt` | Path to banned words file |
| `-log-file` | `proxy.log` | Access log file path |
| `-dial-timeout` | `10s` | Origin dial timeout |
| `-idle-timeout` | `90s` | Keep-alive idle timeout |
| `-verbose` | `false` | Verbose per-request debug logging |

### Filter file format

`forbidden-hosts.txt` and `banned-words.txt`: one entry per line. Blank lines
and `#`-comment lines are ignored; entries are trimmed and lower-cased. A bare
host entry (`example.com`) also blocks its subdomains (`www.example.com`).
Banned-word matching is a case-insensitive substring scan over the response
body. Both files are optional — a missing file disables that filter.

## Architecture

```
client ──▶ http.Server(:8989) ──▶ ProxyHandler.ServeHTTP
                                        │
                       CONNECT? ──yes──▶ tunnel(): hijack conn, dial origin,
                          │              200 Established, io.Copy both ways
                          no
                          ▼
                       forward(): absolute-form host → blocklist check →
                       strip hop-by-hop → X-Forwarded-For → RoundTrip →
                       banned-word scan → relay headers+body → log
```

- `internal/filter` — host blocklist (exact + subdomain suffix match) and
  banned-word body scan. Read-only after load, safe for concurrent use.
- `internal/logx` — access logger over `io.MultiWriter(stdout, file)`; writes
  serialized by the stdlib logger mutex.
- `internal/proxy` — the `http.Handler`: forwarding, filtering, and `CONNECT`
  tunneling with a shared connection-pooling `http.Transport`.
- `main.go` — flag parsing, wiring, graceful shutdown on SIGINT/SIGTERM.

## Tests

All tests are fully offline, using `net/http/httptest` in-process origins
(including a TLS origin for the `CONNECT` tunnel).

```sh
go test ./...
```

Coverage: forward happy-path (+ `X-Forwarded-For`, header relay), hop-by-hop
stripping including `Connection`-token-named headers, host blocking (forward and
`CONNECT`), banned-word blocking vs clean passthrough, origin-down `502`,
malformed origin-form `400`, `CONNECT` TLS round-trip through the tunnel,
`CONNECT` to a blocked/dead host, plus `filter` and `logx` unit tests.

## Notes & Caveats

- **Banned-word scanning buffers the response.** Scanning a body is at odds with
  streaming, so when `banned-words.txt` is non-empty the proxy buffers each
  response (capped at 8 MiB) before serving it; bodies over the cap are blocked
  rather than served unscanned. When no banned words are configured, responses
  stream straight through with bounded memory.
- **`CONNECT` tunnels are blind by design.** The proxy never sees HTTPS
  plaintext. True content filtering of HTTPS would require TLS termination /
  MITM with a CA the client trusts, which is intentionally **not** implemented.
- **`-race` on this Windows toolchain:** the race detector needs cgo, and the
  bundled MinGW gcc here cannot compile 64-bit cgo, so `go test -race` fails to
  build in this environment. `go test ./...` (without `-race`) passes. On a host
  with a working 64-bit C toolchain, `go test -race ./...` runs the same suite
  and exercises the concurrent tunnel goroutines and shared logger.
