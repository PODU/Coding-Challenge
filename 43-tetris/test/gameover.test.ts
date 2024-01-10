import { describe, it, expect } from 'vitest';
import { reduce, emptyGrid } from '../src/core/index';
import { startGame, withGrid } from './helpers';

describe('game over', () => {
  it('triggers when a freshly spawned piece overlaps the top', () => {
    let s = startGame(5);
    const grid = emptyGrid();
    // Fill the spawn zone (cols 3..6, rows 0..1) without completing any row,
    // so the next spawn after a lock is guaranteed to collide.
    for (const c of [3, 4, 5, 6]) {
      grid[0][c] = 'Z';
      grid[1][c] = 'Z';
    }
    s = withGrid(s, grid);
    s = reduce(s, { type: 'HARD_DROP' }); // current piece locks at the bottom, next spawn collides
    expect(s.phase).toBe('gameover');
  });

  it('ignores further play actions once game over', () => {
    let s = startGame(5);
    const grid = emptyGrid();
    for (const c of [3, 4, 5, 6]) {
      grid[0][c] = 'Z';
      grid[1][c] = 'Z';
    }
    s = withGrid(s, grid);
    s = reduce(s, { type: 'HARD_DROP' });
    expect(s.phase).toBe('gameover');

    const snapshot = JSON.stringify(s);
    s = reduce(s, { type: 'MOVE', dir: -1 });
    s = reduce(s, { type: 'ROTATE', dir: 1 });
    s = reduce(s, { type: 'SOFT_DROP' });
    s = reduce(s, { type: 'TICK', dtMs: 5000 });
    expect(JSON.stringify(s)).toBe(snapshot);
  });

  it('RESTART returns a fresh playing state', () => {
    let s = startGame(5);
    const grid = emptyGrid();
    for (const c of [3, 4, 5, 6]) {
      grid[0][c] = 'Z';
      grid[1][c] = 'Z';
    }
    s = withGrid(s, grid);
    s = reduce(s, { type: 'HARD_DROP' });
    expect(s.phase).toBe('gameover');

    s = reduce(s, { type: 'RESTART' });
    expect(s.phase).toBe('playing');
    expect(s.score).toBe(0);
    expect(s.lines).toBe(0);
    expect(s.level).toBe(0);
    expect(s.active).not.toBeNull();
    expect(s.grid.flat().every((c) => c === 0)).toBe(true);
  });
});
