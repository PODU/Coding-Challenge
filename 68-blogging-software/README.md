# Coding Challenge #68 — Blogging Software

A self-hosted blogging engine in Go. It serves a blog dynamically over HTTP:
an authenticated author writes posts in Markdown through a web-based admin
editor; the engine renders them to HTML with server-side templates, lists them
newest-first on a homepage, exposes stable permalinks, accepts visitor
comments, and publishes an RSS feed.


## Features

- Dynamic HTTP server on `:8080` (configurable), pure Go, single static binary.
- Server-side rendering with `html/template` (context-aware auto-escaping) and
  templates embedded via `embed.FS`.
- Markdown authoring rendered to **sanitized** HTML (gomarkdown + bluemonday):
  scripts, `javascript:` URLs, and event handlers are stripped.
- Authenticated admin editor at `/editor`: login, create, and edit posts.
- Draft vs. published lifecycle, with **scheduled** posts (future `published_at`
  stay hidden until their time arrives).
- Stable, id-based permalinks `/posts/{id}` plus auto-generated slugs.
- Public comments (name / email / body); email stored but never rendered;
  bodies HTML-escaped; per-IP rate limiting.
- RSS 2.0 feed at `/feed.xml`.
- Pluggable storage behind a `Store` interface: a file-backed **JSON** store and
  an embedded **SQLite** store (pure-Go `modernc.org/sqlite`), with a one-shot
  JSON→SQLite migration.
- Session auth with HttpOnly/SameSite cookies, bcrypt password hashing, and CSRF
  protection on all editor forms.

## Project layout

```
cmd/blog/             CLI: serve / export / migrate-json-to-sqlite / hash-password
internal/blog/        domain model: Post, Comment, Status, slugify, clock
internal/render/      Markdown -> sanitized HTML pipeline
internal/store/       Store interface, JSONStore, SQLiteStore, migration
internal/feed/        RSS 2.0 builder (encoding/xml)
internal/web/         router, handlers, auth + CSRF middleware
  templates/          embedded html/template files
  static/             embedded CSS
```

## Build

```sh
go build -o blog ./cmd/blog
```

## Run

The editor requires admin credentials. Generate a bcrypt hash for your password,
then start the server:

```sh
# 1. hash a password (prompts on stdin, or pass as an argument)
export ADMIN_USER=admin
export ADMIN_PASSWORD_HASH="$(./blog hash-password)"

# 2. start the server (SQLite store by default)
./blog serve
# open http://localhost:8080  and  http://localhost:8080/editor
```

If `ADMIN_USER`/`ADMIN_PASSWORD_HASH` are unset, the public blog still serves but
the editor is disabled (it returns `503` with a clear message and a startup
warning is logged).

### Configuration

Flags (with defaults) and environment variables:

| Flag | Default | Purpose |
| --- | --- | --- |
| `--addr` | `:8080` | listen address |
| `--store` | `sqlite` | storage backend: `json` or `sqlite` |
| `--data` | `./data` | data directory for the JSON store |
| `--db` | `./blog.db` | database file for the SQLite store |
| `--base-url` | `http://localhost:8080` | public origin used in the RSS feed |
| `--title` / `--description` | — | site metadata |
| `--dev` | `false` | re-parse templates per request (live editing) |

| Env var | Purpose |
| --- | --- |
| `ADMIN_USER` | admin username for the editor |
| `ADMIN_PASSWORD_HASH` | bcrypt hash of the admin password |

## CLI subcommands

```sh
blog serve                          # run the HTTP server (default command)
blog export  [--out ./public]       # render a static snapshot of the public blog
blog migrate-json-to-sqlite \
     [--data ./data] [--db blog.db] # copy a JSON store into SQLite
blog hash-password [password]       # print a bcrypt hash for ADMIN_PASSWORD_HASH
```

Exit codes: `0` ok, `1` runtime error, `2` usage/config error.

## HTTP routes

| Method + Path | Auth | Notes |
| --- | --- | --- |
| `GET /` | no | published posts, newest-first |
| `GET /posts/{id}` | no | permalink; `404` if draft/missing/scheduled |
| `POST /posts/{id}/comments` | no | add comment; `303` on success, `422` on validation error |
| `GET /feed.xml` | no | RSS 2.0 of published posts |
| `GET /editor` | yes | post list (redirects to login if unauthenticated) |
| `GET,POST /editor/login` | no | session login |
| `POST /editor/logout` | yes | clear session |
| `GET,POST /editor/post` | yes | create post |
| `GET,POST /editor/post/{id}` | yes | edit post |
| `GET /static/{file}` | no | embedded CSS |

## Storage tradeoffs

- **JSON store** — one file per post under `data/posts/{id}.json` (+ comments
  under `data/comments/`); zero-dependency, human-diffable, atomic writes via
  temp-file + rename. No transactions and O(n) listing — fine for a single
  author.
- **SQLite store** — transactional, indexed newest-first ordering, concurrent
  reads, still a single embedded file (keeps the single-binary property). This
  is the default. `migrate-json-to-sqlite` copies an existing JSON store across,
  preserving ids, slugs, ordering, and comments.

## Testing

All tests are hermetic: no real network, no bound ports (handlers are driven via
`net/http/httptest`), the clock is injected, and temp dirs come from
`t.TempDir()`. SQLite tests use an in-memory database.

```sh
go test ./...
gofmt -l .   # formatting
go vet ./...
```

Coverage includes: Markdown rendering and sanitization (XSS), slug generation,
JSON/SQLite store round-trips with ordering and status filtering, scheduling,
the JSON→SQLite migration, RSS generation and escaping, and full HTTP handler
flows (homepage ordering, permalink published/draft, auth gate, login + CSRF,
post create/edit with publish-date stamping, and comment submission with XSS
escaping).

## Security notes

- All author and visitor input is escaped by `html/template`; rendered Markdown
  is additionally sanitized through a tag/attribute allowlist.
- Passwords are bcrypt-hashed; sessions use HttpOnly, SameSite=Lax cookies.
- Editor POST forms carry CSRF tokens bound to the session; comment submissions
  are rate-limited per IP.
