/**
 * Signalling server for the video chat (REST plus long polling).
 *
 * It pairs two browsers in a room, relays their offer/answer/ICE/bye messages
 * without looking inside them, and serves the client files. Media never passes
 * through it. Rooms live in memory and are dropped after a period of inactivity.
 */

import { createServer as createHttpServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { randomUUID } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, extname, join, normalize, resolve, sep } from 'node:path';
import type { Server } from 'node:http';
import { parseSignalMessage, type Role, type SignalMessage } from '../shared/messages.js';

export interface ServerOptions {
  /** Max ms a `/poll` request blocks before returning `[]`. Default 25s. */
  pollTimeoutMs?: number;
  /** Evict rooms idle for longer than this. Default 10 min. */
  roomTtlMs?: number;
  /** Run the periodic TTL sweep. Default true; disable in tests. */
  enableSweep?: boolean;
  /** Absolute path of the directory that contains `public/` and `dist/`. */
  rootDir?: string;
}

const DEFAULTS = {
  pollTimeoutMs: 25_000,
  roomTtlMs: 10 * 60_000,
  sweepIntervalMs: 60_000,
} as const;

const MAX_BODY_BYTES = 256 * 1024; // real signalling messages are a few KB
const MAX_PEERS_PER_ROOM = 2;

interface Peer {
  id: string;
  role: Role;
  /** Messages waiting for this peer's next `/poll`. */
  mailbox: SignalMessage[];
  /** Parked long-poll requests. */
  waiters: Array<(msgs: SignalMessage[]) => void>;
}

interface Room {
  id: string;
  peers: Map<string, Peer>;
  /** Messages sent before the other peer joined. */
  pending: SignalMessage[];
  lastActivity: number;
}

function sendJson(res: ServerResponse, status: number, body: unknown): void {
  const text = JSON.stringify(body);
  res.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(text),
  });
  res.end(text);
}

function sendStatus(res: ServerResponse, status: number): void {
  res.writeHead(status);
  res.end();
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolvePromise, reject) => {
    const chunks: Buffer[] = [];
    let size = 0;
    req.on('data', (chunk: Buffer) => {
      size += chunk.length;
      if (size > MAX_BODY_BYTES) {
        reject(new Error('payload too large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolvePromise(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

/** An empty body parses to `undefined`; bad JSON throws. */
function parseJsonBody(text: string): unknown {
  if (text.trim() === '') return undefined;
  return JSON.parse(text) as unknown;
}

const CONTENT_TYPES: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.map': 'application/json; charset=utf-8',
  '.ico': 'image/x-icon',
  '.svg': 'image/svg+xml',
};

/**
 * Resolve a request path to a file on disk, confined to `rootDir`.
 * `/` maps to `public/index.html`; `/dist/...` is served from the build
 * output; everything else is served from `public/`.
 */
function resolveStaticPath(rootDir: string, urlPath: string): string | null {
  const clean = decodeURIComponent(urlPath.split('?')[0] ?? '/');
  let relative: string;
  if (clean === '/' || clean === '') {
    relative = 'public/index.html';
  } else if (clean.startsWith('/dist/')) {
    relative = clean.slice(1);
  } else {
    relative = join('public', clean);
  }
  const full = normalize(resolve(rootDir, relative));
  // Reject anything that ".." took outside rootDir.
  if (full !== rootDir && !full.startsWith(rootDir + sep)) return null;
  return full;
}

async function serveStatic(res: ServerResponse, rootDir: string, urlPath: string): Promise<void> {
  const filePath = resolveStaticPath(rootDir, urlPath);
  if (filePath === null) {
    sendStatus(res, 403);
    return;
  }
  try {
    const data = await readFile(filePath);
    const type = CONTENT_TYPES[extname(filePath)] ?? 'application/octet-stream';
    res.writeHead(200, { 'content-type': type, 'content-length': data.length });
    res.end(data);
  } catch {
    sendStatus(res, 404);
  }
}

class RoomStore {
  private readonly rooms = new Map<string, Room>();

  create(): Room {
    let id = shortId();
    while (this.rooms.has(id)) id = shortId();
    const room: Room = { id, peers: new Map(), pending: [], lastActivity: Date.now() };
    this.rooms.set(id, room);
    return room;
  }

  get(id: string): Room | undefined {
    return this.rooms.get(id);
  }

  delete(id: string): void {
    this.rooms.delete(id);
  }

    sweep(ttlMs: number, now = Date.now()): void {
    for (const [id, room] of this.rooms) {
      if (now - room.lastActivity > ttlMs) this.rooms.delete(id);
    }
  }
}

function shortId(): string {
  return randomUUID().replace(/-/g, '').slice(0, 8);
}

/** Deliver a message to every peer except the sender, waking any long-polls. */
function deliver(room: Room, msg: SignalMessage): void {
  let delivered = false;
  for (const peer of room.peers.values()) {
    if (peer.id === msg.from) continue;
    peer.mailbox.push(msg);
    flushWaiters(peer);
    delivered = true;
  }
  if (!delivered) room.pending.push(msg);
}

function flushWaiters(peer: Peer): void {
  if (peer.mailbox.length === 0 || peer.waiters.length === 0) return;
  const drained = peer.mailbox;
  peer.mailbox = [];
  for (const waiter of peer.waiters) waiter(drained);
  peer.waiters = [];
}

export interface SignalServer {
  http: Server;
  store: RoomStore;
  /** Stop the TTL sweep timer so the process can exit. */
  close(): Promise<void>;
}

export function createSignalServer(options: ServerOptions = {}): SignalServer {
  const pollTimeoutMs = options.pollTimeoutMs ?? DEFAULTS.pollTimeoutMs;
  const roomTtlMs = options.roomTtlMs ?? DEFAULTS.roomTtlMs;
  const enableSweep = options.enableSweep ?? true;
  const rootDir = options.rootDir ?? defaultRootDir();

  const store = new RoomStore();

  const http = createHttpServer((req, res) => {
    handleRequest(req, res).catch((err: unknown) => {
      if (!res.headersSent) sendJson(res, 500, { error: 'internal error' });
      else res.end();
      console.error('[signalling] handler error:', err);
    });
  });

  const sweepTimer = enableSweep
    ? setInterval(() => store.sweep(roomTtlMs), DEFAULTS.sweepIntervalMs)
    : null;
  sweepTimer?.unref();

  async function handleRequest(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const method = req.method ?? 'GET';
    const url = new URL(req.url ?? '/', 'http://localhost');
    const path = url.pathname;

    if (!path.startsWith('/api/')) {
      if (method === 'GET' || method === 'HEAD') {
        await serveStatic(res, rootDir, req.url ?? '/');
      } else {
        sendStatus(res, 405);
      }
      return;
    }

    // /api/rooms
    if (path === '/api/rooms' && method === 'POST') {
      const room = store.create();
      sendJson(res, 201, { roomId: room.id });
      return;
    }

    // /api/rooms/:id/<action>
    const m = /^\/api\/rooms\/([^/]+)\/(join|signal|poll|leave)$/.exec(path);
    if (!m) {
      sendStatus(res, 404);
      return;
    }
    const roomId = decodeURIComponent(m[1] as string);
    const action = m[2] as 'join' | 'signal' | 'poll' | 'leave';
    const room = store.get(roomId);

    if (action === 'poll' && method === 'GET') {
      if (!room) {
        sendStatus(res, 404);
        return;
      }
      handlePoll(room, url, res);
      return;
    }

    if (method !== 'POST') {
      sendStatus(res, 405);
      return;
    }

    let body: unknown;
    try {
      body = parseJsonBody(await readBody(req));
    } catch {
      sendJson(res, 400, { error: 'invalid request body' });
      return;
    }

    if (action === 'join') {
      handleJoin(room, body, res);
    } else if (action === 'signal') {
      handleSignal(room, body, res);
    } else {
      handleLeave(room, roomId, body, res);
    }
  }

  function handleJoin(room: Room | undefined, body: unknown, res: ServerResponse): void {
    if (!room) {
      sendJson(res, 404, { error: 'room not found' });
      return;
    }
    if (room.peers.size >= MAX_PEERS_PER_ROOM) {
      sendJson(res, 409, { error: 'room is full' });
      return;
    }
    const requested = isRecord(body) && typeof body['peerId'] === 'string' ? body['peerId'] : '';
    const peerId = requested !== '' && !room.peers.has(requested) ? requested : randomUUID();
    const role: Role = room.peers.size === 0 ? 'initiator' : 'responder';
    const peer: Peer = { id: peerId, role, mailbox: [...room.pending], waiters: [] };
    room.pending = [];
    room.peers.set(peerId, peer);
    room.lastActivity = Date.now();
    sendJson(res, 200, { peerId, role });
  }

  function handleSignal(room: Room | undefined, body: unknown, res: ServerResponse): void {
    if (!room) {
      sendJson(res, 404, { error: 'room not found' });
      return;
    }
    const msg = parseSignalMessage(body);
    if (!msg) {
      sendJson(res, 400, { error: 'invalid signal message' });
      return;
    }
    deliver(room, msg);
    room.lastActivity = Date.now();
    sendStatus(res, 202);
  }

  function handlePoll(room: Room, url: URL, res: ServerResponse): void {
    const peerId = url.searchParams.get('peerId') ?? '';
    const peer = room.peers.get(peerId);
    if (!peer) {
      sendStatus(res, 404);
      return;
    }
    room.lastActivity = Date.now();

    if (peer.mailbox.length > 0) {
      const drained = peer.mailbox;
      peer.mailbox = [];
      sendJson(res, 200, drained);
      return;
    }

    // Nothing queued: hold the request until a message arrives or we time out.
    let settled = false;
    const finish = (msgs: SignalMessage[]): void => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      sendJson(res, 200, msgs);
    };
    const timer = setTimeout(() => {
      peer.waiters = peer.waiters.filter((w) => w !== finish);
      finish([]);
    }, pollTimeoutMs);
    timer.unref?.();
    peer.waiters.push(finish);

    res.on('close', () => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      peer.waiters = peer.waiters.filter((w) => w !== finish);
    });
  }

  function handleLeave(room: Room | undefined, roomId: string, body: unknown, res: ServerResponse): void {
    if (!room) {
      sendStatus(res, 204);
      return;
    }
    const peerId = isRecord(body) && typeof body['peerId'] === 'string' ? body['peerId'] : '';
    const peer = room.peers.get(peerId);
    if (peer) {
      for (const waiter of peer.waiters) waiter([]);
      peer.waiters = [];
      deliver(room, { from: peerId, kind: 'bye', payload: null });
      room.peers.delete(peerId);
    }
    room.lastActivity = Date.now();
    if (room.peers.size === 0) store.delete(roomId);
    sendStatus(res, 204);
  }

  function close(): Promise<void> {
    if (sweepTimer) clearInterval(sweepTimer);
    return new Promise((done) => http.close(() => done()));
  }

  return { http, store, close };
}

function isRecord(x: unknown): x is Record<string, unknown> {
  return typeof x === 'object' && x !== null;
}

function defaultRootDir(): string {
  // dist/server/server.js -> project root is two levels up.
  const here = dirname(fileURLToPath(import.meta.url));
  return resolve(here, '..', '..');
}

const isMain =
  process.argv[1] !== undefined &&
  resolve(fileURLToPath(import.meta.url)) === resolve(process.argv[1]);

if (isMain) {
  const port = Number(process.env['PORT'] ?? 8080);
  const server = createSignalServer();
  server.http.listen(port, () => {
    console.log(`[signalling] video chat server listening on http://localhost:${port}`);
  });
  process.on('SIGINT', () => {
    void server.close().then(() => process.exit(0));
  });
}
