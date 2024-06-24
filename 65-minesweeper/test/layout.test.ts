import { describe, it, expect } from 'vitest';
import { createBoard, placeMines, reveal } from '../src/core/board';
import { mulberry32 } from '../src/core/rng';
import { mineCoords } from './helpers';

describe('deterministic mine layout', () => {
  it('produces an exact, reproducible mine set for a fixed seed', () => {
    const a = createBoard({ rows: 9, cols: 9, mines: 10 });
    const b = createBoard({ rows: 9, cols: 9, mines: 10 });
    placeMines(a, mulberry32(12345), 0, 0);
    placeMines(b, mulberry32(12345), 0, 0);
    expect(mineCoords(a)).toEqual(mineCoords(b));
  });

  it('different seeds generally yield different layouts', () => {
    const a = createBoard({ rows: 16, cols: 16, mines: 40 });
    const b = createBoard({ rows: 16, cols: 16, mines: 40 });
    placeMines(a, mulberry32(1), 8, 8);
    placeMines(b, mulberry32(2), 8, 8);
    expect(mineCoords(a)).not.toEqual(mineCoords(b));
  });

  it('places exactly `mines` mines', () => {
    const b = createBoard({ rows: 16, cols: 30, mines: 99 });
    placeMines(b, mulberry32(777), 5, 5);
    expect(mineCoords(b).size).toBe(99);
  });

  it('never places a mine on the safe cell or its neighbours', () => {
    for (let seed = 0; seed < 50; seed++) {
      const b = createBoard({ rows: 9, cols: 9, mines: 10 });
      const sr = 4;
      const sc = 4;
      placeMines(b, mulberry32(seed), sr, sc);
      const mines = mineCoords(b);
      for (let dr = -1; dr <= 1; dr++) {
        for (let dc = -1; dc <= 1; dc++) {
          expect(mines.has(`${sr + dr},${sc + dc}`)).toBe(false);
        }
      }
    }
  });

  it('placeMines is a no-op once mines are placed', () => {
    const b = createBoard({ rows: 9, cols: 9, mines: 10 });
    placeMines(b, mulberry32(5), 0, 0);
    const first = mineCoords(b);
    placeMines(b, mulberry32(99), 8, 8);
    expect(mineCoords(b)).toEqual(first);
  });

  it('falls back to excluding only the safe cell when the board is dense', () => {
    // 3x3 with 8 mines: only one safe cell possible, neighbour-exclusion impossible.
    const b = createBoard({ rows: 3, cols: 3, mines: 8 });
    reveal(b, 1, 1, mulberry32(3));
    expect(mineCoords(b).size).toBe(8);
    expect(b.cells[1][1].isMine).toBe(false);
  });
});
