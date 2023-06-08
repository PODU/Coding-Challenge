# Coding Challenge #12 — URL Shortener

A production-ready URL shortening service (like bit.ly / tinyurl) built in Go
with the standard library only. It accepts long URLs, generates short base62
keys, persists the mapping, and serves HTTP redirects.

## Features

- `POST /shorten` — create a short URL (idempotent for repeated URLs).
- `GET /{key}` — 302 redirect to the original URL.
- `DELETE /{key}` — remove a mapping.
- `GET /healthz` — liveness probe.
- `GET /` — minimal web GUI (embedded, served from `web/index.html`).
- Deterministic base62 key generation (SHA-256 truncated) with robust
  collision retry (re-salt + grow key length).
- Pluggable `Store` interface: thread-safe in-memory store by default, or a
  JSON-file–backed persistent store (atomic writes).
- Graceful shutdown, request body limits, and URL validation.

## Layout

```
cmd/server/main.go            Entry point: flags, wiring, graceful shutdown
internal/shortener/
    store.go                  Store interface + MemStore + FileStore (atomic)
    keygen.go                 base62 key generation + collision handling
    handler.go                HTTP handlers (create / redirect / delete / GUI)
    handler_test.go           httptest unit + integration tests
web/
    index.html                Embedded single-page GUI
    web.go                    Embeds index.html
.github/workflows/ci.yml      CI: gofmt, go vet, go test -race
go.mod                        Module (stdlib only)
```

## Run

```sh
# In-memory store (default)
go run ./cmd/server

# Persistent JSON file store, custom address + public base URL
go run ./cmd/server -addr :9000 -base-url https://sho.rt -store ./data/store.json
```

Then open <http://localhost:8080> for the GUI.

### Flags

| Flag         | Default                   | Description                              |
|--------------|---------------------------|------------------------------------------|
| `-addr`      | `:8080`                   | HTTP listen address                      |
| `-base-url`  | `http://localhost:8080`   | External base URL used to build links    |
| `-store`     | `""` (in-memory)          | Path to JSON file for persistent storage |

## API

### `POST /shorten`

Request:

```json
{ "url": "https://example.com/very/long/path" }
```

Responses:

- `201 Created` — new mapping.
- `200 OK` — URL already shortened (idempotent), same body shape.
- `400 Bad Request` — `{ "error": "..." }` for empty/malformed JSON, missing
  scheme, non-http(s) scheme, missing host, or URL over 2048 chars.

```json
{
  "key": "abc1234",
  "long_url": "https://example.com/very/long/path",
  "short_url": "http://localhost:8080/abc1234"
}
```

### `GET /{key}`

- `302 Found` with `Location: <long_url>` for a known key.
- `404 Not Found` for an unknown key.

### `DELETE /{key}`

- `204 No Content` on success.
- `404 Not Found` if the key does not exist.

Unsupported methods on a route return `405 Method Not Allowed`.

## curl smoke test

```sh
# Create
curl -s -X POST localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
# -> {"key":"...","long_url":"https://example.com","short_url":"http://localhost:8080/..."}

# Redirect (expect 302 + Location)
curl -s -i localhost:8080/<key>

# Delete (expect 204)
curl -s -i -X DELETE localhost:8080/<key>
```

## Test

```sh
gofmt -l .          # formatting (no output = clean)
go vet ./...
go build ./...
go test -race ./... # all handlers, redirects, idempotency, collisions, persistence
```

## Design notes / caveats

- **Idempotency** is implemented via a reverse `url -> key` index
  (`FindByURL`). Under highly concurrent submission of the *same* URL there is a
  small window where two keys could be minted for one URL; both resolve
  correctly, but the mapping is not strictly unique-per-URL under that race. The
  data structures themselves are fully mutex-guarded (clean under `-race`).
- **Key space**: 7 base62 chars (~3.5 trillion combinations). Collisions are
  astronomically unlikely; when they do occur the generator re-salts and grows
  the key length until a free key is found.
- **Persistence**: the `FileStore` flushes the whole map on every mutation using
  a temp-file + rename for atomicity. This is simple and crash-safe but O(n) per
  write — fine for the challenge scale, not for millions of links. The `Store`
  interface is the seam to drop in SQLite/Redis without touching handlers.
- Redirect targets are stored and returned verbatim (query strings and
  fragments preserved).
