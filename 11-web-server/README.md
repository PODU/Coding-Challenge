# Coding Challenge #11 — Build Your Own Web Server

A small static HTTP/1.1 web server written in Go on **raw TCP sockets**. It
hand-parses the request line and headers, serves files from a configurable
document root, returns correct status codes and content types, handles many
connections concurrently (goroutine per connection), and blocks
directory-traversal attacks.

## Features

- Raw `net.Listen` / `net.Conn` server loop — no `net/http` server for the core.
- Hand-written request parser: request line (method / target / version) plus
  case-insensitive headers, with byte caps and read deadlines (slow-loris
  defence).
- Static file serving from a configurable doc root with directory-index
  (`index.html`) support.
- Correct status codes: `200`, `400`, `403`, `404`, `405`, `500`.
- Content-Type by file extension (`text/html`, `text/css`,
  `application/javascript`, `image/png`, …) with an `application/octet-stream`
  fallback.
- `GET` and `HEAD` only; other methods get `405` + `Allow: GET, HEAD`.
- Directory-traversal protection: URL-decode → reject non-`/` / NUL / backslash
  → `path.Clean` → join to absolute root → containment check → `403` on escape.
- Goroutine per connection; immutable config means no locking. Per-connection
  panic recovery so one bad request never crashes the process.
- Graceful shutdown on `Ctrl-C` (`SIGINT`/`SIGTERM`): stops accepting, drains
  in-flight connections.

## Layout

```
11-web-server/
├── main.go              # flags, listener, signal handling
├── server/
│   ├── server.go        # Server struct, accept loop, per-conn goroutine
│   ├── request.go       # raw request reader + parser
│   ├── response.go      # response builder / serialisation
│   ├── handler.go       # routing, status codes, content types
│   ├── safepath.go      # path decode/clean/containment check
│   └── *_test.go        # unit + integration tests
└── www/                 # default document root (index.html, about.html, style.css)
```

## Build

```sh
cd 11-web-server
go build -o web-server .      # produces ./web-server (web-server.exe on Windows)
```

## Run

```sh
go run .                          # serve ./www on http://127.0.0.1:8080
go run . -addr :9000 -dir site    # serve ./site on all interfaces, port 9000
./web-server -dir /var/www        # serve a custom doc root

Flags:
  -addr string   host:port to listen on        (default "127.0.0.1:8080")
  -dir  string   document root to serve files   (default "./www")
```

Stop with `Ctrl-C`.

## Examples

With the server running on `127.0.0.1:8080`:

```sh
curl -i http://127.0.0.1:8080/                               # 200, www/index.html
curl -i http://127.0.0.1:8080/style.css                      # 200, text/css
curl -i http://127.0.0.1:8080/nope                           # 404, HTML error page
curl -i --path-as-is http://127.0.0.1:8080/../../etc/passwd  # 403 Forbidden
curl -I http://127.0.0.1:8080/                               # HEAD, headers only
curl -i -X POST http://127.0.0.1:8080/                       # 405 + Allow: GET, HEAD
```

Or open <http://127.0.0.1:8080/> in a browser — the page renders with CSS.

### HTTP behaviour summary

| Request                     | Result                                     |
|-----------------------------|--------------------------------------------|
| `GET /`                     | 200, `www/index.html`                      |
| `GET /index.html`           | 200, `www/index.html`                      |
| `GET /style.css`            | 200, `text/css`                            |
| `GET /missing.html`         | 404, HTML error page                       |
| `GET /../etc/passwd`        | 403 Forbidden (traversal blocked)          |
| `GET /sub/` (has index)     | 200, `www/sub/index.html`                  |
| `GET /sub/` (no index)      | 404                                        |
| `POST /`                    | 405 Method Not Allowed, `Allow: GET, HEAD` |
| `HEAD /index.html`          | 200, headers only, no body                 |
| Malformed request line      | 400 Bad Request                            |

Every response includes `Server`, `Date`, `Content-Type`, `Content-Length` and
`Connection: close`.

## Test

```sh
go test ./...           # run the full suite
go test -race ./...     # with the race detector
go vet ./...
gofmt -l .              # should print nothing
```

The suite covers the request parser (valid / malformed / truncated / duplicate
& case-insensitive headers / byte caps), `safepath.Resolve` (allowed paths and
every traversal vector → `403`), the handler (status-code and content-type
selection, `HEAD`, directory index), and end-to-end integration over a real TCP
listener (content types, 404/405, traversal blocked via raw TCP, 50 concurrent
clients, doc-root validation).

## Notes / caveats

- HTTP/1.1 keep-alive is **not** implemented — every response sets
  `Connection: close` and the connection is closed after one request (matches
  the challenge scope). A real server would reuse the connection.
- No request body handling beyond parsing headers (the server only serves static
  `GET`/`HEAD`); a `POST` body is ignored and answered with `405`.
- No directory listing is generated (by design, to avoid information leakage).
- CGI / dynamic content is an optional extension and is intentionally not
  implemented.
- The default `-addr` is `127.0.0.1:8080` rather than port 80 to avoid needing
  elevated privileges.
