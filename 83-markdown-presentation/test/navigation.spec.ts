import { describe, it, expect } from 'vitest';
import { clampIndex, nextIndex, prevIndex } from '../src/client/navigation.js';

describe('clampIndex', () => {
  it('clamps below 0 to 0', () => {
    expect(clampIndex(-5, 3)).toBe(0);
  });

  it('clamps >= count to count-1', () => {
    expect(clampIndex(10, 3)).toBe(2);
  });

  it('leaves in-range values unchanged', () => {
    expect(clampIndex(1, 3)).toBe(1);
  });

  it('clamps an empty deck (count 0) to 0', () => {
    expect(clampIndex(0, 0)).toBe(0);
    expect(clampIndex(5, 0)).toBe(0);
    expect(clampIndex(-5, 0)).toBe(0);
  });

  it('handles non-finite input', () => {
    expect(clampIndex(NaN, 3)).toBe(0);
    expect(clampIndex(Infinity, 3)).toBe(2);
  });
});

describe('nextIndex / prevIndex (no wrap)', () => {
  it('next advances by one', () => {
    expect(nextIndex(0, 3)).toBe(1);
  });

  it('next at last index stays put', () => {
    expect(nextIndex(2, 3)).toBe(2);
  });

  it('prev goes back by one', () => {
    expect(prevIndex(2, 3)).toBe(1);
  });

  it('prev at first index stays put', () => {
    expect(prevIndex(0, 3)).toBe(0);
  });
});

describe('property: results are always in [0, max(0, count-1)]', () => {
  it('holds for random inputs', () => {
    for (let t = 0; t < 1000; t++) {
      const count = Math.floor(Math.random() * 10);
      const i = Math.floor(Math.random() * 40) - 20;
      const upper = Math.max(0, count - 1);
      for (const fn of [clampIndex, nextIndex, prevIndex]) {
        const r = fn(i, count);
        expect(r).toBeGreaterThanOrEqual(0);
        expect(r).toBeLessThanOrEqual(upper);
      }
    }
  });
});
