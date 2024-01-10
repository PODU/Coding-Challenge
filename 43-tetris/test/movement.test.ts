import { describe, it, expect } from 'vitest';
import { reduce } from '../src/core/index';
import type { ActivePiece } from '../src/core/index';
import { startGame, withActive, withGrid, gridWith } from './helpers';

const O = (row: number, col: number): ActivePiece => ({ id: 'O', rotation: 0, row, col });

describe('movement', () => {
  it('moves left and right in open space', () => {
    let s = withActive(startGame(1), O(5, 4));
    s = reduce(s, { type: 'MOVE', dir: -1 });
    expect(s.active!.col).toBe(3);
    s = reduce(s, { type: 'MOVE', dir: 1 });
    expect(s.active!.col).toBe(4);
  });

  it('is blocked by the left and right walls', () => {
    const left = reduce(withActive(startGame(1), O(5, 0)), { type: 'MOVE', dir: -1 });
    expect(left.active!.col).toBe(0);
    const right = reduce(withActive(startGame(1), O(5, 8)), { type: 'MOVE', dir: 1 });
    expect(right.active!.col).toBe(8);
  });

  it('is blocked by locked cells (stack)', () => {
    let s = startGame(1);
    s = withGrid(s, gridWith([[5, 6], [6, 6]]));
    s = withActive(s, O(5, 4));
    s = reduce(s, { type: 'MOVE', dir: 1 }); // would overlap the stack at col 6
    expect(s.active!.col).toBe(4);
  });

  it('soft drop moves one row down and awards a point', () => {
    let s = withActive(startGame(1), O(5, 4));
    const before = s.score;
    s = reduce(s, { type: 'SOFT_DROP' });
    expect(s.active!.row).toBe(6);
    expect(s.score).toBe(before + 1);
  });

  it('soft drop on a grounded piece is a clean no-op', () => {
    let s = withActive(startGame(1), O(18, 4)); // resting on the floor
    const before = { ...s };
    s = reduce(s, { type: 'SOFT_DROP' });
    expect(s.active!.row).toBe(18);
    expect(s.score).toBe(before.score);
    expect(s.dropAccumulatorMs).toBe(before.dropAccumulatorMs);
  });
});
