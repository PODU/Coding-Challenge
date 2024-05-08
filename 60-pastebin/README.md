# Pastebin

A self-contained Pastebin web service in Go — John Crickett's
[Coding Challenge #60](https://codingchallenges.substack.com/p/coding-challenge-60-pastebin).

Submit a chunk of text, get a unique short URL. Anyone with the URL can view the
paste as a rendered HTML page or as raw text. Pastes can carry an optional
expiry (TTL) after which they vanish, and can be deleted with a secret delete
key. The service exposes both a **REST API** and a minimal **web UI**, persists
durably to SQLite, enforces a maximum paste size, and runs a background sweeper
that reaps expired pastes.

## Features

- Create a paste → unique short URL (random base62 IDs from `crypto/rand`).
- Persist + retrieve by URL (HTML view and raw `text/plain` view).
- `303` redirect to the paste after a web-form submit.
- Idempotent delete with a confirmation step and a secret delete key.
- Optional expiry/TTL with **lazy** read-path checks (via an injectable clock)
  **and** a background **sweeper** goroutine.
- Configurable maximum paste size, enforced at the transport and service layers.
- Pluggable storage behind a `Store` interface: in-memory and SQLite, validated
  by a shared contract test.
- Bonus: syntax-highlight language selection and one-click copy.
- Operability: env-var config, structured JSON logs, `/healthz`, graceful
  shutdown.
- Safety: HTML output auto-escaped by `html/template` (no stored XSS); raw served
  as `text/plain` with `X-Content-Type-Options: nosniff`.

## Quick start

```bash
# Run with the durable SQLite store (creates ./pastebin.db)
go run ./cmd/pastebin

# Or run with an ephemeral in-memory store (great for trying it out)
PASTEBIN_DB=dev go run ./cmd/pastebin
```

Then open <http://localhost:8080>.

### Build a single static binary

```bash
go build -o pastebin ./cmd/pastebin
./pastebin
```

The driver (`modernc.org/sqlite`) is pure Go (CGo-free), so the binary is fully
static and needs no C toolchain.

## Configuration

All configuration is via environment variables:

| Variable | Default | Meaning |
|----------|---------|---------|
| `PASTEBIN_ADDR` | `:8080` | Listen address. |
| `PASTEBIN_DB` | `pastebin.db` | SQLite path. `dev`, `mem`, `memory`, or `:memory:` selects the in-memory store. |
| `PASTEBIN_MAX_BYTES` | `1048576` | Max paste size in bytes (1 MiB). |
| `PASTEBIN_SWEEP_INTERVAL` | `60s` | Background sweeper tick. |
| `PASTEBIN_BASE_URL` | `http://localhost:8080` | Base URL used to build returned links. |
| `PASTEBIN_MAX_TTL` | `0` (unlimited) | Cap on a user-requested TTL. |

## REST API

### Create — `POST /api/pastes`

```bash
curl -X POST http://localhost:8080/api/pastes \
  -H 'Content-Type: application/json' \
  -d '{"content":"hello world","expires_in":"1h","language":"go"}'
```

- `content` (required, non-empty, ≤ max size).
- `expires_in` (optional): a Go duration (`"10m"`, `"1h"`, `"24h"`), or omitted /
  `""` / `"never"` for no expiry. Rejected if it exceeds `PASTEBIN_MAX_TTL`.
- `language` (optional): a highlight token (e.g. `go`, `python`); `""` = plain.

`201 Created`:

```json
{
  "id": "aZ3kP9xQ2bC",
  "url": "http://localhost:8080/aZ3kP9xQ2bC",
  "raw_url": "http://localhost:8080/aZ3kP9xQ2bC/raw",
  "delete_key": "f1c2…e9",
  "expires_at": "2026-06-26T15:00:00Z"
}
```

> Save the `delete_key` — it is required to delete the paste via the API and is
> shown only once.

### Get — `GET /api/pastes/{id}`

`200` with the paste JSON. `404` if it never existed or was deleted; `410 Gone`
if it existed but has expired.

### Raw — `GET /{id}/raw` (or `GET /api/pastes/{id}/raw`)

`200 text/plain; charset=utf-8` with `X-Content-Type-Options: nosniff`, body =
content verbatim. Same `404`/`410` semantics.

### Delete — `DELETE /api/pastes/{id}`

Send the secret key in the `X-Delete-Key` header.

```bash
curl -X DELETE http://localhost:8080/api/pastes/aZ3kP9xQ2bC \
  -H 'X-Delete-Key: f1c2…e9'
```

`204 No Content` on success; `403` if the key is wrong/missing; `404` if the id
is unknown.

### Health — `GET /healthz`

`200 {"status":"ok"}` (also pings the store).

### Status codes

| Situation | Code |
|-----------|------|
| Created | 201 |
| Fetched / raw OK | 200 |
| Deleted | 204 |
| Validation error / bad expiry | 400 |
| Wrong delete key | 403 |
| Unknown id (never existed / deleted) | 404 |
| Expired | 410 |
| Over size limit | 413 |
| Unexpected | 500 |

## Web UI

- `GET /` — new-paste form (content, expiry, language).
- `POST /` — creates the paste and `303`-redirects to `/{id}`. The delete key is
  revealed once to the creator via a short-lived `HttpOnly` cookie.
- `GET /{id}` — view page with Copy / Raw / Delete controls.
- `GET /{id}/raw` — raw text.
- `POST /{id}/delete` — confirmation-guarded delete.

## Architecture

```
cmd/pastebin/         wiring, config, graceful shutdown
internal/config/      env-var configuration
internal/clock/       Clock interface: Real + Fake (injected for tests)
internal/paste/       domain model + Service (validation, ID gen, TTL, size)
internal/store/       Store interface + MemStore + SQLiteStore (+ contract test)
internal/sweep/       background expiry sweeper (RunOnce + Run loop)
internal/highlight/   syntax-highlight language registry (bonus)
internal/httpapi/     REST + web handlers, middleware, router
web/                  embedded templates + static assets (embed.FS)
```

The `Store` interface and the `Clock` interface are injected, so the full HTTP
stack is testable hermetically with an in-memory store and a fake clock — no
network and no wall-clock waiting. The storage interface is Postgres-ready:
swapping in a `PostgresStore` needs no service/handler changes.

## Tests

```bash
go test ./...            # full hermetic suite
go vet ./...
gofmt -l .               # should print nothing
```

Coverage includes: store contract tests (run against both `MemStore` and
`SQLiteStore`), service tests (create→retrieve, raw, expiry via fake clock, size
limit, delete + wrong-key rejection, bad/over-max TTL, ID-collision retry), HTTP
handler tests via `httptest` (create/get/raw/delete, redirect, 404/410/413,
malformed JSON, XSS escaping), and a sweeper test.

> **Note:** `go test -race` requires CGO and a C toolchain. On a machine with a
> 64-bit C compiler, `go test -race ./...` works; the suite itself is
> race-clean.

## License

Provided as-is for the coding challenge.
