import { createHmac, timingSafeEqual } from 'node:crypto';
import type { Request, Response } from 'express';

export const SESSION_COOKIE = 'keep_session';
const MAX_AGE_SECONDS = 60 * 60 * 24 * 30; // 30 days

function sign(value: string, secret: string): string {
  return createHmac('sha256', secret).update(value).digest('base64url');
}

/** Produce a signed cookie payload `value.signature`. */
export function signValue(value: string, secret: string): string {
  return `${value}.${sign(value, secret)}`;
}

/** Verify a signed payload and return the underlying value, or null. */
export function unsignValue(signed: string, secret: string): string | null {
  const dot = signed.lastIndexOf('.');
  if (dot <= 0) return null;
  const value = signed.slice(0, dot);
  const sig = signed.slice(dot + 1);
  const expected = sign(value, secret);
  const a = Buffer.from(sig);
  const b = Buffer.from(expected);
  if (a.length !== b.length) return null;
  return timingSafeEqual(a, b) ? value : null;
}

/** Parse a raw Cookie header into a key/value map. */
export function parseCookies(header: string | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  if (!header) return out;
  for (const part of header.split(';')) {
    const idx = part.indexOf('=');
    if (idx === -1) continue;
    const k = part.slice(0, idx).trim();
    const v = part.slice(idx + 1).trim();
    if (k) out[k] = decodeURIComponent(v);
  }
  return out;
}

export function readSession(req: Request, secret: string): string | null {
  const cookies = parseCookies(req.headers.cookie);
  const raw = cookies[SESSION_COOKIE];
  if (!raw) return null;
  return unsignValue(raw, secret);
}

export function setSession(res: Response, userId: string, secret: string): void {
  const signed = signValue(userId, secret);
  const attrs = [
    `${SESSION_COOKIE}=${encodeURIComponent(signed)}`,
    'Path=/',
    'HttpOnly',
    'SameSite=Lax',
    `Max-Age=${MAX_AGE_SECONDS}`,
  ];
  res.append('Set-Cookie', attrs.join('; '));
}

export function clearSession(res: Response): void {
  const attrs = [`${SESSION_COOKIE}=`, 'Path=/', 'HttpOnly', 'SameSite=Lax', 'Max-Age=0'];
  res.append('Set-Cookie', attrs.join('; '));
}
