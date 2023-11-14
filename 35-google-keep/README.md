# Coding Challenge #35 — Google Keep clone

A production-ready, sticky-note style note-taking app: a **REST API** backend over a
persistent store, **cookie-based authentication** with per-user isolation, and a
served **static SPA** frontend. Full stack in **TypeScript / Node.js**.

Implements the challenge's Steps 0–6 plus the "Going Further" / killer features:
create / view / list / edit / delete notes (id-addressable), **search**, **pin**,
**color**, **archive**, **trash + restore**, and **labels/tags** — all scoped to
the authenticated user.

## Stack

- **Backend:** Express 4 + [zod](https://zod.dev) validation, TypeScript (strict).
- **Persistence:** dependency-free JSON/in-memory store with atomic writes
  (`:memory:` for hermetic tests — no native modules, no external services).
- **Auth:** signed HTTP-only session cookie. Google OAuth 2.0 (injectable client)
  plus a local dev-login path (`AUTH_LOCAL=1`).
- **Frontend:** vanilla TS/JS SPA (masonry note grid, inline editor, search,
  labels, color palette) served statically by the backend — no second build step.
- **Tests:** vitest + supertest, against an in-memory store.

## Project layout

```
35-google-keep/
  src/
    index.ts            bootstrap: config, store, app, graceful shutdown
    app.ts              makeApp(deps) -> Express app (testable factory)
    config.ts           env-driven config
    store/store.ts      in-memory store + JSON file persistence
    repos/              noteRepo.ts labelRepo.ts userRepo.ts  (pure data layer)
    routes/             notes.ts labels.ts auth.ts
    middleware/         auth.ts errors.ts logging.ts
    auth/google.ts      injectable Google OAuth client
    session.ts          signed-cookie sessions
    types.ts            shared domain types / DTOs
  public/               index.html app.js styles.css  (static SPA)
  test/                 noteRepo / labelRepo / notes.api / labels.api / auth.api
  package.json  tsconfig.json  vitest.config.ts
```

## Setup

```bash
cd 35-google-keep
npm install
```

## Run

```bash
npm run build      # tsc -> dist/
npm start          # serves API + SPA on http://localhost:3000
```

Then open <http://localhost:3000>. With `AUTH_LOCAL=1` (the default) you can sign
in from the login screen with any username — handy for local development.

Dev mode (no build, requires the optional `ts-node` loader):

```bash
AUTH_LOCAL=1 npm run dev
```

### Configuration (environment variables)

| Var | Default | Purpose |
|-----|---------|---------|
| `PORT` | `3000` | HTTP port |
| `DATABASE_PATH` | `./data/keep.json` | JSON store path, or `:memory:` |
| `SESSION_SECRET` | dev value | HMAC key for session cookies (set in prod) |
| `AUTH_LOCAL` | `1` | Enable local username login |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | — | Enables Google OAuth |
| `PUBLIC_DIR` | `./public` | Static SPA directory |

## Test

```bash
npm test           # vitest run — hermetic, in-memory store, no network
npm run typecheck  # tsc --noEmit
```

The suite covers the data layer (CRUD, search, ordering, pin/archive/trash
transitions, label cascade, per-user isolation, OAuth upsert idempotency) and the
API handlers (auth, validation, cross-user 404s, 401 on missing session, label
auto-create, faked Google OAuth callback).

## API summary

All endpoints are under `/api`; every `/api/notes*` and `/api/labels*` request is
scoped to the session user. Errors use a consistent envelope:
`{ "error": { "code", "message" } }`.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/auth/me` | Current user or 401 |
| POST | `/api/auth/local` | Dev login `{ username }` |
| GET | `/api/auth/google` · `/api/auth/google/callback` | Google OAuth |
| POST | `/api/auth/logout` | Clear session |
| POST | `/api/notes` | Create `{ title?, content?, color?, labels?, ... }` |
| GET | `/api/notes/:id` | Get one (404 if not owned) |
| GET | `/api/notes` | List; `?q= &label= &archived= &trashed= &pinned= &sort= &limit= &offset=` |
| PATCH | `/api/notes/:id` | Partial update |
| DELETE | `/api/notes/:id` | Soft-delete (trash); `?purge=1` hard-deletes |
| POST | `/api/notes/:id/restore` | Un-trash / un-archive |
| GET·POST | `/api/labels` | List / create labels |
| DELETE | `/api/labels/:id` | Delete label (cascades) |

## Caveats

- Persistence uses a JSON file (the design's allowed alternative to SQLite). This
  keeps the project dependency-light and avoids native build steps; the repository
  seam means swapping in SQLite later is mechanical.
- Google OAuth requires real credentials to exercise end-to-end; the callback flow
  is covered in tests via an injected fake client (no live Google call).
- Concurrency is last-write-wins (v1), as specified in the design.
