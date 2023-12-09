# Coding Challenge #39 — Social Media Tool

A Buffer/Hootsuite-style backend that lets you **create, store, schedule, edit and
publish** posts to social networks (LinkedIn, X), built around a
**message-queue + worker** pipeline. Single Go binary, no external broker required.

This covers challenge steps 0–5.

---

## Architecture

```
HTTP client / Web UI
        │
        ▼
   REST API (net/http)  ──create/edit/list/cancel/publish──▶  Store (SQLite)
        │                                                        ▲   │
        │ immediate posts enqueued                               │   │ claim due
        ▼                                                        │   ▼
   Queue (interface: in-process channel | broker)  ◀──enqueue── Scheduler (ticker + injectable clock)
        │
        ▼
   Publisher worker pool (goroutines)
        │  Provider.Publish()  + bounded retry/backoff + idempotency
        ▼
   Provider (interface): mock | linkedin | x  ──▶ external API
```

Every seam is an interface so the parts are swappable and hermetically testable:

| Concern        | Interface                | Default impl            | Test impl                  |
|----------------|--------------------------|-------------------------|----------------------------|
| Persistence    | `store.Store`            | SQLite (`modernc.org/sqlite`, pure Go) | `:memory:` SQLite |
| Message queue  | `queue.Queue`            | in-process channel      | same (bounded, hermetic)   |
| Publishing     | `provider.Provider`      | `linkedin`, `x`         | `mock` (records calls, no network) |
| Time           | `clock.Clock`            | system clock            | `clock.Fake` (manual advance) |

### Why a DB-polling scheduler (approach A)

The scheduler claims due posts with a transactional
`scheduled → queued` compare-and-set (`ClaimDue`), so a post is dispatched
**exactly once** even with multiple scheduler instances, survives restarts, and
can be **edited/canceled any time before it is claimed** (Step 5). The
queue-TTL/delayed-delivery alternative (approach B) would be the option at larger scale.

---

## Layout

```
cmd/server/main.go        wires store, queue, scheduler, workers, HTTP; graceful shutdown
internal/
  model/      Post, Status/Visibility enums, validation
  store/      Store interface + SQLite impl (transactional lifecycle ops)
  queue/      Queue interface + in-process channel queue (bounded)
  provider/   Provider interface + registry; mock, linkedin, x
  scheduler/  ticker loop + claim-due dispatch (injectable clock)
  publisher/  worker pool: consume → publish → status, with retry/backoff/idempotency
  api/        REST handlers (net/http)
  config/     env-driven config
  clock/      Clock abstraction (system + fake)
  e2e/        in-process end-to-end tests
migrations/0001_init.sql  schema (also applied programmatically on startup)
web/index.html            Step 4/5 vanilla-JS UI
```

---

## Run

```bash
go run ./cmd/server          # listens on :8080, SQLite at ./social.db, mock provider
# open http://localhost:8080/ for the web UI
```

Configuration (environment variables, all optional):

| Var                    | Default | Meaning                                   |
|------------------------|---------|-------------------------------------------|
| `SMT_ADDR`             | `:8080` | HTTP listen address                       |
| `SMT_DSN`              | `social.db` | SQLite path (use `:memory:` for ephemeral) |
| `SMT_WORKERS`          | `4`     | publisher worker goroutines               |
| `SMT_QUEUE_CAPACITY`   | `256`   | bounded queue size (backpressure)         |
| `SMT_SCHEDULER_TICK`   | `5s`    | scheduler poll interval                   |
| `SMT_MAX_ATTEMPTS`     | `3`     | publish attempts before `failed`          |
| `SMT_TARGETS`          | `mock`  | comma list of providers: `mock,linkedin,x`|
| `SMT_LINKEDIN_TOKEN` / `SMT_LINKEDIN_AUTHOR` | – | LinkedIn OAuth token + author URN (Step 0) |
| `SMT_X_TOKEN`          | –       | X OAuth bearer token (Step 0)             |

The real `linkedin`/`x` providers require Step 0 credentials and perform network
I/O; they are **never** exercised by the test suite.

---

## REST API

Base path `/api/v1`. JSON bodies, RFC 3339 UTC times. Errors: `{"error": "..."}`.

| Method | Path                          | Purpose | Success |
|--------|-------------------------------|---------|---------|
| `POST`   | `/api/v1/posts`             | Create. `scheduled_at` omitted → publish now (`queued`); set → `scheduled`. | `201` + post |
| `GET`    | `/api/v1/posts`             | List. Query: `status`, `target`, `author`, `limit`, `offset`. | `200` + `{items,total}` |
| `GET`    | `/api/v1/posts/{id}`        | Fetch one. | `200` / `404` |
| `PATCH`  | `/api/v1/posts/{id}`        | Edit `body`/`visibility`/`scheduled_at` (Step 5). Only while draft/scheduled. | `200` / `409` |
| `DELETE` | `/api/v1/posts/{id}`        | Cancel a draft/scheduled post. | `204` / `409` |
| `POST`   | `/api/v1/posts/{id}/publish`| Force immediate publish. | `202` |
| `GET`    | `/healthz`                  | Liveness (DB ping). | `200` |

### Example

```bash
# schedule a post
curl -s localhost:8080/api/v1/posts -d '{
  "author":"acme-co","body":"Launching today!","visibility":"public",
  "target":"mock","scheduled_at":"2026-07-01T09:00:00Z"}'

# publish it now instead
curl -s -X POST localhost:8080/api/v1/posts/<id>/publish

# edit a scheduled post
curl -s -X PATCH localhost:8080/api/v1/posts/<id> -d '{"body":"Updated copy"}'
```

### Validation
- `author`, `body`, `target` required; `target ∈ {mock, linkedin, x}`.
- `visibility ∈ {public, connections, private}` (default `public`).
- `body` non-empty and ≤ network limit (X 280, LinkedIn 3000, mock 5000),
  enforced by `Provider.Validate` at create **and** edit.
- `scheduled_at`, if present, must be in the future (UTC).
- `PATCH`/`DELETE` return `409` once a post is `queued`/`publishing`/`published`.

---

## Post lifecycle

```
draft ─(create immediate)─▶ queued ─▶ publishing ─▶ published
  │                                       └(error, attempts<max)─▶ queued (retry, backoff)
  │                                       └(error, attempts=max | auth)─▶ failed
  └(create with scheduled_at)─▶ scheduled ─(due)─▶ queued ─▶ ...
scheduled ─(DELETE/cancel before due)─▶ canceled
```

- **Idempotency:** once `published` (or otherwise out of `queued`), a re-delivered
  message is rejected by `BeginPublish`, so retries never double-post.
- **Retry:** transient failures re-enqueue with bounded exponential backoff + jitter;
  auth failures and exhausted attempts go straight to `failed`.
- **Cancel race:** a worker re-checks status atomically before publishing, so a
  post canceled after enqueue is dropped.

---

## Test

```bash
go test ./...            # full hermetic suite — no network, no Docker
go vet ./...
gofmt -l .               # should print nothing
```

Coverage maps to the design's test plan:

- **queue** — FIFO ordering, bounded backpressure, graceful close/drain.
- **model** — validation table tests.
- **store** — CRUD round-trip, due-claim selects only due rows and claims each
  **exactly once under concurrency**, edit guard, full publish lifecycle.
- **scheduler** — fake clock; dispatches only due posts; dispatches after the
  clock advances (no real waiting); skips canceled posts.
- **publisher** — consume→publish; retry-then-succeed; fail after max attempts;
  auth error not retried; canceled message dropped; idempotent re-delivery.
- **api** — create (immediate vs scheduled), validation `400`s, get/`404`,
  list filters, patch + `409`, delete `204`, publish-now `202`, `/healthz`.
- **e2e** — boots the whole pipeline in-process: immediate publish, scheduled
  publish via clock advance, edit-then-publish.

> **Note on `-race`:** the suite is concurrency-safe by construction (channels +
> mutexes). The Go race detector needs cgo with a 64-bit C compiler, which is not
> available on this Windows dev box (only 32-bit MinGW gcc), so `go test -race`
> cannot link here. On a machine with a 64-bit gcc/clang, `go test ./... -race`
> runs clean.

---

## Mapping to challenge steps

| Step | Where |
|------|-------|
| 1 — queue listener publishes to networks | `queue/`, `provider/`, `publisher/` |
| 2 — persist post details | `store/`, `migrations/`, `POST`/`GET` API |
| 3 — scheduling service | `scheduler/`, `scheduled_at`, `clock/` |
| 4 — UI to create + schedule/post-now | `web/index.html` |
| 5 — edit scheduled posts | `PATCH`/`DELETE` API + UI editing |
| 0 / extensions — real providers, Postgres, auth | `provider/linkedin.go`, `provider/x.go`; design §4.7, §8 |
