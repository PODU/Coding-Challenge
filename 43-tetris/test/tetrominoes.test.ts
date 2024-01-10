import { describe, it, expect } from 'vitest';
import { SHAPES, PIECES } from '../src/core/index';

describe('tetromino tables', () => {
  it('defines all seven pieces', () => {
    expect(PIECES).toHaveLength(7);
    expect(new Set(PIECES).size).toBe(7);
  });

  it('each piece has 4 rotation states of exactly 4 cells', () => {
    for (const id of PIECES) {
      const states = SHAPES[id];
      expect(states).toHaveLength(4);
      for (const state of states) {
        expect(state).toHaveLength(4);
      }
    }
  });

  it('offsets stay within the bounding box (non-negative, < 4)', () => {
    for (const id of PIECES) {
      for (const state of SHAPES[id]) {
        for (const { r, c } of state) {
          expect(r).toBeGreaterThanOrEqual(0);
          expect(c).toBeGreaterThanOrEqual(0);
          expect(r).toBeLessThan(4);
          expect(c).toBeLessThan(4);
        }
      }
    }
  });

  it('O piece is rotation-invariant', () => {
    const [a, b, c, d] = SHAPES.O;
    expect(b).toEqual(a);
    expect(c).toEqual(a);
    expect(d).toEqual(a);
  });
});
