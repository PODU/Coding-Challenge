# Notion Clone — Coding Challenge #69

A full-stack, block-based document editor in the spirit of [Notion](https://www.notion.so).
Sign in, create **pages**, and fill each page with an ordered, nestable tree of
**blocks** (paragraphs, headings, lists, to-dos, quotes, dividers) with inline
**bold/italic**. Everything persists and is served through a REST API that a
single-page web editor consumes.


---

## Stack

- **Backend** — Node 20 + **Express**, **zod** request validation, layered
  `routes → services → store`. Auth uses **`node:crypto`** only: scrypt password
  hashing and HMAC-SHA256 signed JWT-style tokens (no native `bcrypt`, no
  `jsonwebtoken` dependency).
- **Data layer** — a pure-JS store behind a single `Store` interface. In-memory
  Maps with snapshot-based transactions, and durable JSON-file persistence when
  a file path is configured. (See [Why not SQLite?](#why-not-sqlite) — this is the
  documented fallback the brief asks for.)
- **Frontend** — a **vanilla TypeScript SPA** (no framework), bundled with
  **esbuild** (prebuilt binary, no native compile). A `contentEditable` block
  editor with the Notion keyboard model.
- **Tests** — **Vitest** + **supertest**, fully hermetic against isolated
  in-memory stores.

Everything builds with `tsc` (strict) and runs on Node 20 with **no native build
chain** — `npm install` pulls zero packages that need a C/C++ compiler.

---

## Quick start

```bash
npm install
npm run build      # typecheck + compile server (tsc) + bundle web (esbuild)
npm start          # serves API + web UI on http://localhost:3000
```

Open <http://localhost:3000>, register an account, and start writing.

Development helper (rebuilds web, compiles server, runs it):

```bash
npm run dev
```

### Environment variables

| Var                  | Default              | Purpose                                   |
| -------------------- | -------------------- | ----------------------------------------- |
| `PORT`               | `3000`               | HTTP port                                 |
| `JWT_SECRET`         | dev placeholder      | Token signing secret (set in production)  |
| `JWT_EXPIRY_SECONDS` | `604800` (7 days)    | Token lifetime                            |
| `DATA_PATH`          | `data/notion.json`   | JSON store path, or `:memory:` ephemeral  |
| `AI_ENABLED`         | `false`              | Enable the bonus Ollama proxy (R11)       |
| `OLLAMA_HOST`        | `http://localhost:11434` | Ollama endpoint for `/api/ai/complete` |

---

## Tests

```bash
npm test
```

46 tests across 7 files, all hermetic (no external services, isolated
`:memory:` store per app):

- **`unit-tree.test.ts`** — pure block-tree logic: fractional ordering, float
  underflow → renormalise, tree assembly, subtree collection, depth, cycle
  detection, marks clamping/merging, type defaults.
- **`service-blocks.test.ts`** — BlockService: create/reorder, indent (move),
  cycle rejection, subtree delete, change-type (text/marks preserved, props
  reset, children retained), to-do checkbox, batch atomicity + rollback.
- **`api-auth.test.ts`** — register/login/me, wrong password (401), duplicate
  email (409), validation envelope.
- **`api-pages.test.ts`** — create (default "Untitled" + seed block), rename,
  nest, duplicate deep-copy ("Copy of …"), reorder, delete cascade, cross-user
  404.
- **`api-blocks.test.ts`** — full CRUD over HTTP, move/indent, change type,
  subtree delete, batch, cross-user 404, unknown-type validation.
- **`edge.test.ts`** — cross-page parent, self-/descendant page nesting (409),
  max nesting depth, duplicate placement order, empty-page seed block, store
  isolation.
- **`persistence.test.ts`** — write via the service, reopen a fresh file-backed
  store, read data back (R10 durability).

---

## API

Base path `/api`; JSON; `Authorization: Bearer <token>` on everything except
register/login. Errors use the envelope
`{ "error": { "code", "message", "details" } }` with codes
`VALIDATION_ERROR` (400), `UNAUTHENTICATED` (401), `FORBIDDEN` (403),
`NOT_FOUND` (404), `CONFLICT` (409), `INTERNAL` (500).

### Auth

| Method | Path                 | Body                  |
| ------ | -------------------- | --------------------- |
| POST   | `/api/auth/register` | `{email, password}`   |
| POST   | `/api/auth/login`    | `{email, password}`   |
| GET    | `/api/auth/me`       | —                     |

### Pages

| Method | Path                         | Notes                                |
| ------ | ---------------------------- | ------------------------------------ |
| GET    | `/api/pages`                 | flat list (client builds the tree)   |
| POST   | `/api/pages`                 | `{title?, parentPageId?}`            |
| GET    | `/api/pages/:id`             | `{page, blocks: <nested tree>}`      |
| PATCH  | `/api/pages/:id`             | rename / nest / set position         |
| POST   | `/api/pages/:id/duplicate`   | deep-copy → "Copy of …"              |
| POST   | `/api/pages/:id/reorder`     | `{afterPageId?, parentPageId?}`      |
| DELETE | `/api/pages/:id`             | cascade to child pages + blocks      |

### Blocks

| Method | Path                              | Notes                              |
| ------ | --------------------------------- | ---------------------------------- |
| POST   | `/api/pages/:pageId/blocks`       | `{type, text?, parentId?, afterBlockId?, props?}` |
| PATCH  | `/api/blocks/:id`                 | `{text?, marks?, props?}`          |
| POST   | `/api/blocks/:id/type`            | `{type}` — change type in place    |
| POST   | `/api/blocks/:id/move`            | `{parentId?, afterBlockId?}`       |
| DELETE | `/api/blocks/:id`                 | deletes the block **and subtree**  |
| POST   | `/api/pages/:pageId/blocks/batch` | `{ops:[…]}` atomic batch           |

Block types: `paragraph`, `heading_1`, `heading_2`, `heading_3`,
`bulleted_list_item`, `numbered_list_item`, `to_do`, `quote`, `divider`.

---

## Editor keyboard model

- **Enter** — new sibling block (continues a list; an empty list item exits to paragraph).
- **Tab / Shift-Tab** — indent / outdent (reparent the block).
- **Backspace** at the start of an empty leaf block — delete it.
- **`/`** — slash menu to change the block type.
- **Ctrl/Cmd-B / -I** — toggle bold / italic on the selection.
- **Checkbox** on a to-do toggles `props.checked`.

Text edits save optimistically (debounced PATCH); structural operations call
their dedicated endpoints and re-sync the page tree.

---

## Project layout

```
69-notion-clone/
├── shared/types.ts            # Block/Page/DTO types shared by server + web
├── server/
│   ├── src/
│   │   ├── index.ts           # bootstrap + graceful shutdown
│   │   ├── app.ts             # Express factory + makeTestApp()
│   │   ├── store.ts           # pure-JS Store (in-memory + JSON file, txns)
│   │   ├── domain/            # ordering, block-tree ops, zod schemas
│   │   ├── services/          # Auth / Page / Block services (invariants)
│   │   ├── routes/            # auth, pages, blocks, ai
│   │   ├── http.ts            # validation, auth middleware, error envelope
│   │   └── util/crypto.ts     # scrypt + HMAC tokens (no native deps)
│   └── test/                  # vitest: unit + integration (46 tests)
├── web/
│   ├── index.html · styles.css
│   └── src/                   # api client, richtext, dom helpers, main SPA
└── scripts/build-web.mjs      # esbuild bundle
```

---

## Why not SQLite?

The original plan used `better-sqlite3`, a **native** addon that requires a
C/C++ build toolchain (Visual Studio Build Tools on Windows) to compile. To keep
the project installable and the test-suite green on a stock Node-only host, the
data layer is implemented as a **pure-JS store behind the same `Store`
interface** — exactly the fallback the task brief calls for. It keeps data in
memory (Maps) with snapshot-based, all-or-nothing transactions for the
multi-row operations (reorder, move, duplicate, subtree delete, renormalise),
and durably persists a JSON snapshot to `DATA_PATH` in production. Swapping in a
SQLite-backed `Store` later is a drop-in: implement the same interface in
`server/src/store.ts`.

## Notes & caveats

- **Persistence** is a JSON snapshot written on each committed change and on
  graceful shutdown. Great for a single-user local app; for high write volume or
  multi-process deployment, slot a SQLite/Postgres `Store` behind the interface.
- The **AI endpoint** (`/api/ai/complete`, bonus R11) is feature-flagged off by
  default and proxies to a local Ollama instance when `AI_ENABLED=true`.
- Inline rich text supports **bold/italic** (the challenge's R5 minimum) via the
  `marks` model; it is intentionally not a full rich-text engine.
