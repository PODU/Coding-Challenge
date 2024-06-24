import { describe, it, expect } from 'vitest';
import { reveal, toggleFlag, revealedCount } from '../src/core/board';
import { mulberry32 } from '../src/core/rng';
import { boardFromLayout, mineCoords } from './helpers';

const noRng = mulberry32(0);

describe('win / lose', () => {
  it('revealing a mine loses the game and reveals all mines', () => {
    const b = boardFromLayout(`
*.*
...
*.*
`);
    reveal(b, 0, 0, noRng);
    expect(b.phase).toBe('lost');
    for (const key of mineCoords(b)) {
      const [r, c] = key.split(',').map(Number);
      expect(b.cells[r][c].state).toBe('revealed');
    }
  });

  it('revealing the final safe cell wins, in any order', () => {
    // Mine-dense layout where every safe cell is numbered (no flood), so each
    // reveal opens exactly one cell and we can assert the win-on-last boundary.
    const b = boardFromLayout(`
*.*
.*.
*.*
`);
    const safe: Array<[number, number]> = [];
    for (let r = 0; r < 3; r++) {
      for (let c = 0; c < 3; c++) {
        if (!b.cells[r][c].isMine) safe.push([r, c]);
      }
    }
    expect(safe.length).toBe(4);
    for (let i = 0; i < safe.length; i++) {
      expect(b.phase).not.toBe('won');
      reveal(b, safe[i][0], safe[i][1], noRng);
    }
    expect(b.phase).toBe('won');
    expect(revealedCount(b)).toBe(4);
  });

  it('no further moves change a finished board', () => {
    const b = boardFromLayout(`
*.
..
`);
    reveal(b, 0, 0, noRng); // lose
    expect(b.phase).toBe('lost');
    const before = revealedCount(b);
    reveal(b, 1, 1, noRng); // no-op
    toggleFlag(b, 1, 1); // no-op
    expect(revealedCount(b)).toBe(before);
    expect(b.flagsPlaced).toBe(0);
  });

  it('a one-click win is possible on a board that is one big zero region', () => {
    const b = boardFromLayout(`
....
....
....
...*
`);
    reveal(b, 0, 0, noRng);
    expect(b.phase).toBe('won');
  });
});
