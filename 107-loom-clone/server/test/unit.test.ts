import { describe, it, expect } from 'vitest';
import { parseRange } from '../src/range';
import { newShareId, newBlobKey } from '../src/ids';

describe('parseRange', () => {
  const size = 1000;

  it('returns null when no Range header', () => {
    expect(parseRange(undefined, size)).toBeNull();
    expect(parseRange('', size)).toBeNull();
  });

  it('parses a closed range', () => {
    expect(parseRange('bytes=0-99', size)).toEqual({ start: 0, end: 99 });
    expect(parseRange('bytes=100-199', size)).toEqual({ start: 100, end: 199 });
  });

  it('parses an open-ended range', () => {
    expect(parseRange('bytes=500-', size)).toEqual({ start: 500, end: 999 });
  });

  it('parses a suffix range', () => {
    expect(parseRange('bytes=-200', size)).toEqual({ start: 800, end: 999 });
  });

  it('clamps end past EOF', () => {
    expect(parseRange('bytes=900-5000', size)).toEqual({ start: 900, end: 999 });
  });

  it('rejects malformed and unsatisfiable ranges', () => {
    expect(parseRange('bytes=abc', size)).toBe('invalid');
    expect(parseRange('items=0-99', size)).toBe('invalid');
    expect(parseRange('bytes=-', size)).toBe('invalid');
    expect(parseRange('bytes=1000-1001', size)).toBe('invalid'); // start >= size
    expect(parseRange('bytes=500-400', size)).toBe('invalid'); // start > end
    expect(parseRange('bytes=-0', size)).toBe('invalid'); // zero suffix
  });
});

describe('share ids', () => {
  it('produces 22-char url-safe unguessable tokens', () => {
    const a = newShareId();
    const b = newShareId();
    expect(a).toHaveLength(22);
    expect(a).toMatch(/^[A-Za-z0-9_-]{22}$/);
    expect(a).not.toBe(b);
  });

  it('produces unique blob keys', () => {
    const seen = new Set<string>();
    for (let i = 0; i < 1000; i += 1) {
      seen.add(newBlobKey());
    }
    expect(seen.size).toBe(1000);
  });
});
