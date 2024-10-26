/**
 * Signalling message types, shared by the server and the browser client so the
 * two can't disagree about the wire format.
 */

export type SignalKind = 'offer' | 'answer' | 'ice' | 'bye';

/** Role assigned by the server on join. The initiator creates the first offer. */
export type Role = 'initiator' | 'responder';

/**
 * A message relayed as-is to the other peer. `payload` is an
 * `RTCSessionDescriptionInit` for offer/answer, an `RTCIceCandidateInit` for
 * ice, and `null` for bye.
 */
export interface SignalMessage {
  /** Sender's peerId; the server delivers to everyone else. */
  from: string;
  kind: SignalKind;
  payload: unknown;
}

/** Response body of `POST /api/rooms`. */
export interface CreateRoomResponse {
  roomId: string;
}

/** Response body of `POST /api/rooms/:id/join`. */
export interface JoinResponse {
  peerId: string;
  role: Role;
}

export const SIGNAL_KINDS: readonly SignalKind[] = ['offer', 'answer', 'ice', 'bye'];

export function isSignalKind(x: unknown): x is SignalKind {
  return typeof x === 'string' && (SIGNAL_KINDS as readonly string[]).includes(x);
}

/** Validates an untrusted request body, returning `null` if it isn't a SignalMessage. */
export function parseSignalMessage(raw: unknown): SignalMessage | null {
  if (typeof raw !== 'object' || raw === null) return null;
  const o = raw as Record<string, unknown>;
  if (typeof o['from'] !== 'string' || o['from'].length === 0) return null;
  if (!isSignalKind(o['kind'])) return null;
  if (!('payload' in o)) return null;
  return { from: o['from'], kind: o['kind'], payload: o['payload'] ?? null };
}
