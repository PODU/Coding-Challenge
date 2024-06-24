import { describe, it, expect } from 'vitest';
import { createBoard, reveal, placeMines } from '../src/core/board';
import { mulberry32 } from '../src/core/rng';

describe('first-click safe (R10)', () => {
  it('mines are not placed until the first reveal', () => {
    const b = createBoard({ rows: 9, cols: 9, mines: 10 });
    expect(b.minesPlaced).toBe(false);
    expect(b.phase).toBe('ready');
    reveal(b, 3, 3, mulberry32(42));
    expect(b.minesPlaced).toBe(true);
    expect(b.phase === 'playing' || b.phase === 'won').toBe(true);
  });

  it('the first revealed cell is never a mine, across many seeds and positions', () => {
    for (let seed = 0; seed < 200; seed++) {
      const r = seed % 9;
      const c = (seed * 7) % 9;
      const b = createBoard({ rows: 9, cols: 9, mines: 10 });
      reveal(b, r, c, mulberry32(seed));
      expect(b.cells[r][c].isMine).toBe(false);
    }
  });

  it('a cell that WOULD be a mine for one safe-origin is still safe when clicked first', () => {
    // Find a seed/target where placing mines around a different origin lands a
    // mine on `target`; then click `target` first and confirm lazy placement
    // keeps it safe.
    const target: [number, number] = [4, 4];
    const otherOrigin: [number, number] = [0, 0];
    let found = false;
    for (let seed = 0; seed < 500 && !found; seed++) {
      const probe = createBoard({ rows: 9, cols: 9, mines: 20 });
      placeMines(probe, mulberry32(seed), otherOrigin[0], otherOrigin[1]);
      if (probe.cells[target[0]][target[1]].isMine) {
        found = true;
        // Now play a real game clicking target first with the same seed.
        const game = createBoard({ rows: 9, cols: 9, mines: 20 });
        reveal(game, target[0], target[1], mulberry32(seed));
        expect(game.cells[target[0]][target[1]].isMine).toBe(false);
      }
    }
    expect(found).toBe(true);
  });
});
