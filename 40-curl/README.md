# cc-curl

A tiny HTTP/1.1 client built for John Crickett's [Coding Challenge #40/#41 — Build your own `curl`](https://codingchallenges.substack.com/p/coding-challenge-41-curl).

The point of the challenge is to **speak HTTP/1.1 by hand over a raw TCP socket** — no
`net/http`. `cc-curl` parses the URL itself, builds the request line and headers as text,
opens a raw `net.Conn` (or a TLS connection for `https`), writes the bytes, and parses the
status line, headers and body (including chunked transfer-encoding) itself. The only
sanctioned shortcut is `crypto/tls` for HTTPS.

## Build

```sh
go build -o cc-curl ./cmd/cc-curl
```

Requires Go 1.24+. Zero external dependencies (standard library only).

## Usage

```
cc-curl [options] <url>

Options:
  -X, --request <method>   HTTP method (GET default; POST if -d and no -X)
  -H, --header <header>    Add/override a header "Name: value" (repeatable)
  -d, --data <data>        Request body; implies POST unless -X given
  -v, --verbose            Print request/response metadata to stderr (> < *)
  -i, --include            Include response headers in the stdout output
  -o, --output <file>      Write body to <file> instead of stdout
  -L, --location           Follow 3xx redirects (max 20)
  -k, --insecure           Skip TLS certificate verification (https)
      --max-redirs <n>     Override redirect limit (default 20)
  -h, --help               Show this help
```

Argument parsing is hand-rolled and accepts `-H x`, `-Hx`, `--header x`, `--header=x`,
combined short boolean flags (`-vL`), and repeated flags.

### Examples

```sh
cc-curl http://eu.httpbin.org/get
cc-curl -v http://eu.httpbin.org/get
cc-curl -X DELETE http://eu.httpbin.org/delete
cc-curl -d '{"k":"v"}' -H 'Content-Type: application/json' http://eu.httpbin.org/post
cc-curl -X PUT -d 'payload' http://eu.httpbin.org/put
cc-curl -v https://eu.httpbin.org/get
cc-curl -L http://eu.httpbin.org/redirect/2
```

Without `-v`, only the response body is printed to stdout. With `-v`, request lines are
prefixed `> `, response lines `< `, and connection/TLS notes `* ` — all on stderr.

## Behaviour notes

- **URL** — `scheme://host[:port]/path?query`; default port 80 (http) / 443 (https);
  missing path becomes `/`; IPv6 literals (`http://[::1]:8080/`) are supported.
- **Request** — always sends `Host`, `Accept: */*`, and `Connection: close`. The `Host`
  header omits the port when it is the scheme default. A body (`-d`) adds `Content-Length`
  and a default `Content-Type: application/x-www-form-urlencoded` unless you supply your
  own. Any `-H` header overrides a default case-insensitively.
- **Method** — `GET` by default, `POST` when `-d` is given without `-X`; `-X` sets any
  method (including `HEAD`, `PATCH`, or custom verbs).
- **Response** — decodes `Transfer-Encoding: chunked`, honours `Content-Length`, or reads
  to EOF (relying on `Connection: close`). `HEAD` and `204`/`304` responses read no body.
  Bare-LF line endings are tolerated on parse.
- **Redirects** — `-L` follows `301/302/303/307/308`; `Location` is resolved relative to
  the current URL; `301/302/303` switch the method to `GET` and drop the body (curl's
  default); bounded by `--max-redirs` (default 20).

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (any HTTP status 2xx–5xx still exits 0, like curl) |
| 1 | Usage error (bad args/URL) |
| 2 | Connection/transport error (DNS, dial, TLS handshake) |
| 3 | Malformed response (parse failure) |

## Architecture

```
cmd/cc-curl/main.go      entry point: parse args -> build -> Client.Do -> render
internal/cli             hand-rolled argv -> Options
internal/urlparse        ParseURL + relative-reference Resolve (no net/url)
internal/httpmsg         BuildRequest (wire bytes) + ParseResponse (status/headers/body/chunked)
internal/transport       Dial: net.Dial (http) / tls.Dial (https), with verbose trace
internal/client          Client.Do: orchestration + redirect following + tracing
internal/trace           verbose "> " / "< " / "* " writer
```

## Tests

```sh
go test ./...
```

All tests run **offline** — HTTP behaviour is exercised against in-process
`net/http/httptest` servers (and `httptest.NewTLSServer` for the TLS path), so there is no
external network dependency. Coverage includes:

- `urlparse` — defaults, ports, queries, fragments, IPv6, errors, relative-reference resolution.
- `httpmsg` — golden request bytes (GET/POST/header-override/Host-override) and response
  parsing (Content-Length, multi-chunk + trailer, read-to-EOF, HEAD, no-reason status,
  bare-LF, duplicate headers).
- `cli` — every flag form, combined clusters, `--flag=value`, repeated `-H`, error cases.
- `client` — full request/response cycle: GET, `-X DELETE`, POST `-d` (Content-Length /
  Content-Type), custom `-H`, chunked decode, `-L` redirect (incl. 303 POST->GET and loop
  guard), verbose trace, HTTPS, HEAD.

## Out of scope

Keep-alive/connection reuse, HTTP/2-3, gzip, cookies, auth, and hand-written TLS — by design.
