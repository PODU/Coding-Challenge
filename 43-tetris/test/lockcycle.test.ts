import { describe, it, expect } from 'vitest';
import { reduce, getGhostRow, cellsOf } from '../src/core/index';
import type { ActivePiece } from '../src/core/index';
import { startGame, withActive } from './helpers';

const O = (row: number, col: number): ActivePiece => ({ id: 'O', rotation: 0, row, col });

describe('lock cycle', () => {
  it('gravity TICK locks the piece at the right moment', () => {
    let s = withActive(startGame(1), O(17, 4)); // one row above the floor
    s = { ...s, gravityMs: 100, dropAccumulatorMs: 0 };

    // Not enough time to step yet.
    s = reduce(s, { type: 'TICK', dtMs: 50 });
    expect(s.active!.id).toBe('O');
    expect(s.active!.row).toBe(17);

    // Cross the threshold once -> moves to the floor (row 18).
    s = reduce(s, { type: 'TICK', dtMs: 60 });
    expect(s.active!.row).toBe(18);

    // Next step cannot move down -> locks; a new piece spawns at the top.
    s = reduce(s, { type: 'TICK', dtMs: 100 });
    expect(s.active!.row).toBe(0);
    expect(s.grid[18][4]).not.toBe(0); // locked cells present
    expect(s.grid[19][4]).not.toBe(0);
  });

  it('HARD_DROP lands exactly on the ghost row and locks atomically', () => {
    let s = withActive(startGame(1), O(0, 4));
    const ghost = getGhostRow(s);
    expect(ghost).toBe(18);

    const before = s.score;
    s = reduce(s, { type: 'HARD_DROP' });
    // O dropped from row 0 to row 18 = 18 cells * 2 points.
    expect(s.score).toBe(before + 36);
    expect(s.grid[18][4]).toBe('O');
    expect(s.grid[18][5]).toBe('O');
    expect(s.grid[19][4]).toBe('O');
    expect(s.grid[19][5]).toBe('O');
  });

  it('ghost row equals the lock row for a single-cell gap', () => {
    let s = withActive(startGame(1), O(0, 0));
    const ghost = getGhostRow(s);
    const landed: ActivePiece = { ...s.active!, row: ghost };
    for (const { r } of cellsOf(landed)) {
      expect(r).toBeLessThanOrEqual(19);
    }
    s = reduce(s, { type: 'HARD_DROP' });
    expect(s.grid[19][0]).toBe('O');
  });
});
