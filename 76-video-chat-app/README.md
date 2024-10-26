# Video Chat Application — Coding Challenge #76

A small, peer-to-peer video chat that runs in the browser. Two people connect
through a room, then stream **audio + video directly to each other** over WebRTC
(`RTCPeerConnection`). The media path is end-to-end encrypted by WebRTC
(DTLS-SRTP) and never touches the server — the server only relays the small
signalling messages (SDP offers/answers and ICE candidates) needed to open the
connection. A text chat over an `RTCDataChannel` validates the P2P link.

## Architecture

```
  Browser A  <──REST/long-poll──>  Signalling server (Node/TS)  <──REST/long-poll──>  Browser B
      └──────────────── direct P2P media + data (WebRTC, encrypted) ─────────────────┘
```

- **Signalling server** (`src/server/server.ts`) — zero runtime dependencies,
  Node standard library only. In-memory rooms (max 2 peers), a per-peer mailbox,
  and a REST + HTTP long-poll relay. Also serves the static client.
- **Shared protocol** (`src/shared/messages.ts`) — the signalling envelope types
  and pure validators, imported by both server and client.
- **Browser client** (`src/client/main.ts` + `public/`) — `getUserMedia`,
  `RTCPeerConnection` with Google STUN, trickle ICE, `ontrack` rendering, and
  the mute / camera / hang-up controls + data-channel chat.

### REST signalling API

| Method | Path | Body | Response |
|--------|------|------|----------|
| POST | `/api/rooms` | – | `201 { roomId }` |
| POST | `/api/rooms/:id/join` | `{ peerId? }` | `200 { peerId, role }` · `404` · `409` (full) |
| POST | `/api/rooms/:id/signal` | `{ from, kind, payload }` | `202` · `404` · `400` |
| GET  | `/api/rooms/:id/poll?peerId=` | – | `200 [messages]` (long-poll, `[]` on timeout) |
| POST | `/api/rooms/:id/leave` | `{ peerId }` | `204` |

`kind` ∈ `offer | answer | ice | bye`. `payload` is the `RTCSessionDescriptionInit`
for offer/answer, the `RTCIceCandidateInit` for ice, and `null` for bye.

## Requirements

- Node 20+ and npm (verified with Node 20.17, npm 10.8).

## Install, build, test

```bash
npm install
npm run build     # tsc (strict) -> dist/
npm test          # vitest: signalling server + protocol helpers
```

## Run it

```bash
npm start                 # serves http://localhost:8080
# or: PORT=3000 npm start
# or: npm run dev         # build + start in one step
```

Then, to test the full call:

1. Open <http://localhost:8080> in **tab 1** and click **Create room**. A room id
   appears (and is added to the URL as `?room=…`). Grant camera/mic access.
2. Copy the room id (or the whole URL) into **tab 2** (or another device on the
   same origin), put it in the room field, and click **Join**. Grant access.
3. Each tab shows its own muted local preview plus the remote video/audio.
4. Try **Mute**, **Camera off**, the **chat** box, and **Hang up**.

> `getUserMedia` only works from a **secure context**: `https://…` or
> `http://localhost`. Opening the file directly or over plain `http` to a LAN IP
> will block camera access.

## Tests

`npm test` runs the deterministic, offline suite (Vitest) against an in-process
server on an OS-assigned port. Two fake peers exercise the full relay:

- room creation + id uniqueness,
- role assignment (initiator / responder) and the third-peer `409`,
- unknown-room `404`,
- offer/answer/ICE delivered to the other peer exactly once, in FIFO order,
  never echoed to the sender, including signals buffered before the second join,
- `bye` on leave and slot re-use,
- long-poll timeout returns `[]`, and a parked poll wakes immediately on a new
  message,
- malformed JSON / invalid `kind` → `400` (server stays up),
- protocol round-trip + validator unit tests.

The WebRTC/`getUserMedia` layer needs a real browser, so it is manual-test only
(it still builds under `tsc --strict`).

## Caveats

- **TURN:** STUN (`stun.l.google.com:19302`) covers most NATs, but symmetric
  NATs / restrictive firewalls need a TURN server. A config slot is left in
  `src/client/main.ts` (`RTC_CONFIG`).
- **State is in-memory:** rooms live in the server process and are evicted after
  ~10 min of inactivity; a restart drops all rooms.
- **Two participants per room** by design.
