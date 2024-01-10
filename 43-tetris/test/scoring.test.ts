import { describe, it, expect } from 'vitest';
import { lineScore, nextLevel, gravityForLevel } from '../src/core/index';

describe('scoring', () => {
  it('uses the NES base table at level 0', () => {
    expect(lineScore(1, 0)).toBe(40);
    expect(lineScore(2, 0)).toBe(100);
    expect(lineScore(3, 0)).toBe(300);
    expect(lineScore(4, 0)).toBe(1200);
  });

  it('scales by (level + 1)', () => {
    expect(lineScore(1, 1)).toBe(80);
    expect(lineScore(4, 2)).toBe(3600);
    expect(lineScore(2, 9)).toBe(1000);
  });

  it('awards nothing for zero lines', () => {
    expect(lineScore(0, 5)).toBe(0);
  });
});

describe('level progression', () => {
  it('advances every 10 lines', () => {
    expect(nextLevel(0)).toBe(0);
    expect(nextLevel(9)).toBe(0);
    expect(nextLevel(10)).toBe(1);
    expect(nextLevel(25)).toBe(2);
    expect(nextLevel(100)).toBe(10);
  });
});

describe('gravity speed', () => {
  it('starts around 800ms at level 0', () => {
    expect(gravityForLevel(0)).toBe(800);
  });

  it('strictly decreases until it clamps to the floor', () => {
    let prev = gravityForLevel(0);
    let clamped = false;
    for (let level = 1; level <= 30; level++) {
      const cur = gravityForLevel(level);
      if (cur === prev) {
        clamped = true; // reached the floor
      } else {
        expect(cur).toBeLessThan(prev);
        expect(clamped).toBe(false); // must not increase after clamping
      }
      prev = cur;
    }
    expect(clamped).toBe(true);
    expect(gravityForLevel(99)).toBe(50);
  });
});
