/**
 * Signalling server tests — deterministic, offline, in-process.
 *
 * Two fake peers exchange offer/answer/ICE through a real server instance bound
 * to an OS-assigned port. No browser, no network, no STUN/TURN.
 */

import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { AddressInfo } from 'node:net';
import { createSignalServer, type SignalServer } from '../src/server/server.js';
import {
  isSignalKind,
  parseSignalMessage,
  type JoinResponse,
  type SignalKind,
  type SignalMessage,
} from '../src/shared/messages.js';

let server: SignalServer;
let base: string;

beforeEach(async () => {
  // Short poll timeout keeps the timeout test fast; disable the TTL sweep.
  server = createSignalServer({ pollTimeoutMs: 200, enableSweep: false });
  await new Promise<void>((done) => server.http.listen(0, done));
  const port = (server.http.address() as AddressInfo).port;
  base = `http://127.0.0.1:${port}`;
});

afterEach(async () => {
  await server.close();
});

async function createRoom(): Promise<string> {
  const res = await fetch(`${base}/api/rooms`, { method: 'POST' });
  expect(res.status).toBe(201);
  return ((await res.json()) as { roomId: string }).roomId;
}

async function join(roomId: string): Promise<{ status: number; body: JoinResponse | null }> {
  const res = await fetch(`${base}/api/rooms/${roomId}/join`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({}),
  });
  const body = res.status === 200 ? ((await res.json()) as JoinResponse) : null;
  return { status: res.status, body };
}

function signal(roomId: string, msg: SignalMessage): Promise<Response> {
  return fetch(`${base}/api/rooms/${roomId}/signal`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(msg),
  });
}

async function poll(roomId: string, peerId: string): Promise<SignalMessage[]> {
  const res = await fetch(`${base}/api/rooms/${roomId}/poll?peerId=${peerId}`);
  expect(res.status).toBe(200);
  return (await res.json()) as SignalMessage[];
}

describe('room creation', () => {
  it('returns a unique roomId across many calls', async () => {
    const ids = new Set<string>();
    for (let i = 0; i < 200; i++) ids.add(await createRoom());
    expect(ids.size).toBe(200);
  });
});

describe('join', () => {
  it('assigns initiator then responder then rejects a third with 409', async () => {
    const roomId = await createRoom();
    const a = await join(roomId);
    const b = await join(roomId);
    const c = await join(roomId);
    expect(a.body?.role).toBe('initiator');
    expect(b.body?.role).toBe('responder');
    expect(a.body?.peerId).not.toBe(b.body?.peerId);
    expect(c.status).toBe(409);
  });

  it('returns 404 for an unknown room', async () => {
    const { status } = await join('does-not-exist');
    expect(status).toBe(404);
  });
});

describe('leave', () => {
  it('frees a slot so a new peer can take it', async () => {
    const roomId = await createRoom();
    const a = await join(roomId);
    await join(roomId); // room now full
    expect((await join(roomId)).status).toBe(409);

    const res = await fetch(`${base}/api/rooms/${roomId}/leave`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ peerId: a.body?.peerId }),
    });
    expect(res.status).toBe(204);

    // The freed slot can be re-joined.
    expect((await join(roomId)).status).toBe(200);
  });
});

describe('signal relay', () => {
  it('delivers messages to the other peer exactly once, in FIFO order, never echoed', async () => {
    const roomId = await createRoom();
    const a = (await join(roomId)).body!;
    const b = (await join(roomId)).body!;

    const offer: SignalMessage = { from: a.peerId, kind: 'offer', payload: { type: 'offer', sdp: 'x' } };
    const ice1: SignalMessage = { from: a.peerId, kind: 'ice', payload: { candidate: 'c1' } };
    const ice2: SignalMessage = { from: a.peerId, kind: 'ice', payload: { candidate: 'c2' } };

    expect((await signal(roomId, offer)).status).toBe(202);
    expect((await signal(roomId, ice1)).status).toBe(202);
    expect((await signal(roomId, ice2)).status).toBe(202);

    const received = await poll(roomId, b.peerId);
    expect(received.map((m) => m.kind)).toEqual(['offer', 'ice', 'ice']);
    expect((received[1]?.payload as { candidate: string }).candidate).toBe('c1');
    expect((received[2]?.payload as { candidate: string }).candidate).toBe('c2');

    // Sender never receives its own messages; its mailbox is empty (times out).
    const echoed = await poll(roomId, a.peerId);
    expect(echoed).toEqual([]);

    // Messages were delivered once — a re-poll for B yields nothing.
    const again = await poll(roomId, b.peerId);
    expect(again).toEqual([]);
  });

  it('buffers signals sent before the second peer joins', async () => {
    const roomId = await createRoom();
    const a = (await join(roomId)).body!;
    // A sends an offer before B has joined.
    await signal(roomId, { from: a.peerId, kind: 'offer', payload: { type: 'offer', sdp: 's' } });

    const b = (await join(roomId)).body!;
    const received = await poll(roomId, b.peerId);
    expect(received).toHaveLength(1);
    expect(received[0]?.kind).toBe('offer');
  });

  it('returns 404 when signalling an unknown room', async () => {
    const res = await signal('nope', { from: 'x', kind: 'bye', payload: null });
    expect(res.status).toBe(404);
  });

  it('delivers a bye to the remaining peer when one leaves', async () => {
    const roomId = await createRoom();
    const a = (await join(roomId)).body!;
    const b = (await join(roomId)).body!;
    await fetch(`${base}/api/rooms/${roomId}/leave`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ peerId: a.peerId }),
    });
    const received = await poll(roomId, b.peerId);
    expect(received.map((m) => m.kind)).toEqual(['bye']);
  });
});

describe('long-poll', () => {
  it('returns [] after the configured timeout', async () => {
    const roomId = await createRoom();
    const b = (await join(roomId)).body!;
    const start = Date.now();
    const msgs = await poll(roomId, b.peerId);
    const elapsed = Date.now() - start;
    expect(msgs).toEqual([]);
    expect(elapsed).toBeGreaterThanOrEqual(150); // ~pollTimeoutMs (200)
    expect(elapsed).toBeLessThan(2000); // and not hanging forever
  });

  it('wakes a parked poll as soon as a message arrives', async () => {
    const roomId = await createRoom();
    const a = (await join(roomId)).body!;
    const b = (await join(roomId)).body!;

    const start = Date.now();
    const polled = poll(roomId, b.peerId); // parks (mailbox empty)
    await signal(roomId, { from: a.peerId, kind: 'answer', payload: { type: 'answer', sdp: 'y' } });
    const received = await polled;
    const elapsed = Date.now() - start;

    expect(received.map((m) => m.kind)).toEqual(['answer']);
    expect(elapsed).toBeLessThan(200); // resolved before the 200ms timeout
  });
});

describe('input validation', () => {
  it('rejects malformed JSON with 400 and stays up', async () => {
    const res = await fetch(`${base}/api/rooms/whatever/join`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: '{ not json',
    });
    expect(res.status).toBe(400);
    // Server still serves subsequent requests.
    expect((await fetch(`${base}/api/rooms`, { method: 'POST' })).status).toBe(201);
  });

  it('rejects a signal with a missing/invalid kind with 400', async () => {
    const roomId = await createRoom();
    await join(roomId);
    const res = await fetch(`${base}/api/rooms/${roomId}/signal`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ from: 'a', kind: 'bogus', payload: null }),
    });
    expect(res.status).toBe(400);
  });

  it('returns 404 for unknown routes', async () => {
    expect((await fetch(`${base}/api/nope`)).status).toBe(404);
  });
});

describe('message protocol helpers', () => {
  it('round-trips every signalling kind through JSON', () => {
    const samples: SignalMessage[] = [
      { from: 'p1', kind: 'offer', payload: { type: 'offer', sdp: 'v=0...' } },
      { from: 'p1', kind: 'answer', payload: { type: 'answer', sdp: 'v=0...' } },
      { from: 'p2', kind: 'ice', payload: { candidate: 'candidate:...', sdpMid: '0' } },
      { from: 'p2', kind: 'bye', payload: null },
    ];
    for (const msg of samples) {
      const parsed = parseSignalMessage(JSON.parse(JSON.stringify(msg)));
      expect(parsed).toEqual(msg);
    }
  });

  it('isSignalKind accepts the four kinds and rejects others', () => {
    for (const k of ['offer', 'answer', 'ice', 'bye'] satisfies SignalKind[]) {
      expect(isSignalKind(k)).toBe(true);
    }
    expect(isSignalKind('candidate')).toBe(false);
    expect(isSignalKind(42)).toBe(false);
    expect(isSignalKind(null)).toBe(false);
  });

  it('parseSignalMessage rejects malformed inputs', () => {
    expect(parseSignalMessage(null)).toBeNull();
    expect(parseSignalMessage({ kind: 'offer', payload: null })).toBeNull(); // no from
    expect(parseSignalMessage({ from: '', kind: 'offer', payload: null })).toBeNull();
    expect(parseSignalMessage({ from: 'a', kind: 'nope', payload: null })).toBeNull();
    expect(parseSignalMessage({ from: 'a', kind: 'offer' })).toBeNull(); // no payload key
    // bye with explicit null payload is valid.
    expect(parseSignalMessage({ from: 'a', kind: 'bye', payload: null })).toEqual({
      from: 'a',
      kind: 'bye',
      payload: null,
    });
  });
});
