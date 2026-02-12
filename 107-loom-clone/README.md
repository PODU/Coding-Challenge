# Loom Clone — Coding Challenge #107

A full-stack, production-shaped Loom clone in TypeScript/Node.

- **Browser recorder** — pick camera/mic/screen, toggle mic & camera live, optional
  picture-in-picture compositing, capture with `MediaRecorder`, and upload in resumable
  chunks while recording.
- **Backend** — Node/Express, chunked + resumable uploads streamed to disk behind a
  pluggable `Storage` interface, a metadata repo, a REST API, unguessable share links,
  and HTTP **Range** streaming for an HTML5 `<video>` player.

---

## Requirements

- Node 20+ and npm on PATH (developed against Node v20.17 / npm 10.8).

## Install

```bash
npm install
```

## Build

```bash
npm run build       # tsc build of the server + typecheck of the browser client
npm run build:web   # bundle the browser client to web/dist (Vite)
```

`npm run build` compiles the backend to `dist/` and type-checks the browser code with
the DOM libs. `npm run build:web` emits the production browser bundle that the server
serves statically.

## Test

```bash
npm test
```

Vitest + Supertest drive the Express app in-process against a temp-dir / in-memory
`Storage` and repo — fully hermetic, no network, no browser. Covered:

- Upload lifecycle: create → append chunks → complete; assembled blob equals the
  concatenated chunks; `shareId` minted; service **streams** into storage (never buffers).
- Resume: `nextExpectedIndex` after a partial upload; reconnect and finish.
- Chunk ordering: gap → `409`; duplicate index ignored, no corruption.
- Size limit → `413`; empty completion → `400`; idempotent re-complete.
- List/get: newest-first ordering and cursor pagination.
- **Range streaming**: `206` partial slices with correct `Content-Range` /
  `Content-Length` / `Accept-Ranges`; rangeless `200` full body; unsatisfiable → `416`.
- Share resolution: public subset only (internal `id` / blob key never leaked); unknown
  `shareId` → `404`; `404` after delete.
- Delete removes blob + metadata, is idempotent, and `404`s subsequent get/stream.
- Validation (Zod) → `400`; unguessable 22-char share token.

## Run

Two processes in development (Vite proxies `/api` to the backend):

```bash
# terminal 1 — backend on :3000
npm run dev

# terminal 2 — browser client with HMR on :5173
npm run dev:web
```

Open <http://localhost:5173>.

Single-process / production-style:

```bash
npm run build:web   # produce web/dist
npm run build       # produce dist/
npm start           # serves API + the built client on :3000
```

Open <http://localhost:3000>.

Environment variables: `PORT` (default `3000`), `DATA_DIR` (default `./data`),
`MAX_UPLOAD_BYTES` (default 2 GiB). Blobs live in `data/blobs/`, upload sessions in
`data/uploads/`, metadata in `data/metadata.json` (all gitignored).

---

## Record in the browser (manual test)

> Screen/camera/mic capture uses Web Platform APIs (`getUserMedia`,
> `getDisplayMedia`, `MediaRecorder`) that only run in a real browser with user
> permission, so it is documented manual test rather than automated. Chrome/Edge give
> the fullest support; Firefox differs on PiP/codecs.

1. Go to **/record**. Grant camera/microphone permission when prompted (this also
   reveals device labels in the dropdowns).
2. Choose a camera and microphone; tick **Record screen** to include a screen/window.
3. Toggle **Camera**/**Microphone** on or off — works before *and* during recording.
4. Click **Start recording** (pick a screen/window if screen capture is on). Chunks
   upload live as you record.
5. Click **Stop** (or use the browser's own "Stop sharing" button — recording ends
   cleanly). The share link appears when the upload finalizes.
6. Open the share link (**/v/:shareId**) — the recording streams back with seeking via
   HTTP Range. The home page (**/**) lists all recordings with open / copy-link / delete.

Things worth exercising manually: permission-denied shows a friendly inline error;
camera-off mid-record keeps the screen feed; dropping the network mid-upload and
restoring it resumes from the next expected chunk; seeking the player exercises Range
streaming end-to-end.

---

## API

Base path `/api`. Errors are `{ "error": { "code", "message" } }`.

| Method | Path | Purpose |
|--------|------|---------|
| POST   | `/uploads` | Begin a resumable upload session → `{ uploadId }` |
| PUT    | `/uploads/:uploadId/chunk?index=N` | Append chunk N (raw octet-stream → disk) → `204` |
| GET    | `/uploads/:uploadId` | Resume info `{ receivedChunks, nextExpectedIndex }` |
| POST   | `/uploads/:uploadId/complete` | Finalize → `Recording` + `shareUrl` |
| GET    | `/recordings?limit&cursor` | List recordings (newest-first) |
| GET    | `/recordings/:id` | Recording metadata |
| GET    | `/recordings/:id/stream` | Range-enabled playback (`206`/`200`) |
| DELETE | `/recordings/:id` | Delete blob + metadata (`204`, idempotent) |
| GET    | `/share/:shareId` | Resolve share link → public subset |
| GET    | `/share/:shareId/stream` | Public Range-enabled streaming |

## Project layout

```
107-loom-clone/
  shared/types.ts          # DTOs imported by both server and web
  server/src/
    app.ts                 # Express app factory (injected deps; used by tests)
    index.ts               # http listen + wiring
    routes/                # uploads, recordings, share
    services/              # UploadService, RecordingService, ShareService
    storage/               # Storage interface, LocalDiskStorage, MockStorage
    repo/Recordings.ts     # metadata repo (in-memory + JSON file)
    http/stream.ts         # Range streaming
    range.ts, errors.ts, ids.ts
  server/test/             # vitest + supertest
  web/                     # index/record/v pages + recorder/uploader/player TS
  vite.config.ts, vitest.config.ts, tsconfig.json, web/tsconfig.json
```

## Out of scope

Accounts/auth, teams, transcription, comments, analytics, server-side transcoding —
noted as future work behind the existing `Storage`/repo interfaces (e.g. an `S3Storage`
drop-in). The browser's native webm/mp4 is stored as-is.
