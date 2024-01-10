import { describe, it, expect } from 'vitest';
import { reduce, emptyGrid, gravityForLevel } from '../src/core/index';
import type { ActivePiece, Grid } from '../src/core/index';
import { startGame, withActive, withGrid } from './helpers';

/** A row filled with 'I' except the given empty columns. */
function rowExcept(empty: number[]): Grid[number] {
  const row = new Array(10).fill('I') as Grid[number];
  for (const c of empty) row[c] = 0;
  return row;
}

describe('engine integration: clears, scoring, level', () => {
  it('a single line clear awards 40 * (level + 1) and counts the line', () => {
    let s = startGame(1);
    const grid = emptyGrid();
    grid[19] = rowExcept([0, 1]); // bottom row needs columns 0 and 1
    s = withGrid(s, grid);
    s = withActive(s, { id: 'O', rotation: 0, row: 0, col: 0 });

    s = reduce(s, { type: 'HARD_DROP' });
    expect(s.lines).toBe(1);
    // 18 cells hard-dropped (36 pts) + 40 for the single line.
    expect(s.score).toBe(36 + 40);
  });

  it('clears a Tetris and scores 1200 at level 0', () => {
    let s = startGame(1);
    const grid = emptyGrid();
    for (const r of [16, 17, 18, 19]) grid[r] = rowExcept([0]); // a 1-wide well at col 0
    s = withGrid(s, grid);
    const verticalI: ActivePiece = { id: 'I', rotation: 3, row: 0, col: -1 };
    s = withActive(s, verticalI);

    s = reduce(s, { type: 'HARD_DROP' });
    expect(s.lines).toBe(4);
    // 16 cells hard-dropped (32 pts) + 1200 for the tetris.
    expect(s.score).toBe(32 + 1200);
  });

  it('levels up and speeds up gravity when crossing a 10-line boundary', () => {
    let s = startGame(1);
    const grid = emptyGrid();
    grid[18] = rowExcept([0, 1]);
    grid[19] = rowExcept([0, 1]);
    s = withGrid(s, grid);
    s = { ...s, lines: 8 }; // an O double clear will reach 10 lines
    s = withActive(s, { id: 'O', rotation: 0, row: 0, col: 0 });

    s = reduce(s, { type: 'HARD_DROP' });
    expect(s.lines).toBe(10);
    expect(s.level).toBe(1);
    expect(s.gravityMs).toBe(gravityForLevel(1));
  });

  it('pause freezes gravity; resume continues', () => {
    let s = withActive(startGame(1), { id: 'O', rotation: 0, row: 5, col: 4 });
    s = { ...s, gravityMs: 100, dropAccumulatorMs: 0 };
    s = reduce(s, { type: 'PAUSE' });
    s = reduce(s, { type: 'TICK', dtMs: 1000 });
    expect(s.active!.row).toBe(5); // unchanged while paused
    s = reduce(s, { type: 'RESUME' });
    s = reduce(s, { type: 'TICK', dtMs: 100 });
    expect(s.active!.row).toBe(6);
  });
});
