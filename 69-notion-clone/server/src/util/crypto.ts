/**
 * Password hashing (scrypt) and HMAC-SHA256 signed tokens in the style of a
 * JWT, using only node:crypto so there is nothing native to build.
 */
import {
  randomBytes,
  randomUUID,
  scryptSync,
  timingSafeEqual,
  createHmac,
} from 'node:crypto';

export function newId(): string {
  return randomUUID();
}

const SCRYPT_KEYLEN = 64;

export function hashPassword(password: string): string {
  const salt = randomBytes(16).toString('hex');
  const derived = scryptSync(password, salt, SCRYPT_KEYLEN).toString('hex');
  return `scrypt$${salt}$${derived}`;
}

export function verifyPassword(password: string, stored: string): boolean {
  const parts = stored.split('$');
  if (parts.length !== 3 || parts[0] !== 'scrypt') return false;
  const [, salt, expected] = parts;
  const derived = scryptSync(password, salt, SCRYPT_KEYLEN);
  const expectedBuf = Buffer.from(expected, 'hex');
  if (expectedBuf.length !== derived.length) return false;
  return timingSafeEqual(derived, expectedBuf);
}

function base64url(input: Buffer | string): string {
  return Buffer.from(input)
    .toString('base64')
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_');
}

function base64urlJson(obj: unknown): string {
  return base64url(JSON.stringify(obj));
}

export interface TokenPayload {
  sub: string;
  iat: number;
  exp: number;
}

export function signToken(
  sub: string,
  secret: string,
  expirySeconds: number,
): string {
  const now = Math.floor(Date.now() / 1000);
  const payload: TokenPayload = { sub, iat: now, exp: now + expirySeconds };
  const header = base64urlJson({ alg: 'HS256', typ: 'JWT' });
  const body = base64urlJson(payload);
  const data = `${header}.${body}`;
  const sig = base64url(createHmac('sha256', secret).update(data).digest());
  return `${data}.${sig}`;
}

export function verifyToken(
  token: string,
  secret: string,
): TokenPayload | null {
  const parts = token.split('.');
  if (parts.length !== 3) return null;
  const [header, body, sig] = parts;
  const data = `${header}.${body}`;
  const expected = base64url(
    createHmac('sha256', secret).update(data).digest(),
  );
  const sigBuf = Buffer.from(sig);
  const expBuf = Buffer.from(expected);
  if (sigBuf.length !== expBuf.length || !timingSafeEqual(sigBuf, expBuf)) {
    return null;
  }
  let payload: TokenPayload;
  try {
    payload = JSON.parse(
      Buffer.from(body.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString(
        'utf8',
      ),
    ) as TokenPayload;
  } catch {
    return null;
  }
  const now = Math.floor(Date.now() / 1000);
  if (typeof payload.exp !== 'number' || payload.exp < now) return null;
  return payload;
}
