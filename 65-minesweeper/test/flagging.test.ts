import { describe, it, expect } from 'vitest';
import { reveal, toggleFlag, minesRemaining } from '../src/core/board';
import { mulberry32 } from '../src/core/rng';
import { boardFromLayout } from './helpers';

const noRng = mulberry32(0);

describe('flagging', () => {
  it('toggles hidden -> flagged -> hidden and tracks counts', () => {
    const b = boardFromLayout(`
*..
...
..*
`);
    expect(minesRemaining(b)).toBe(2);
    toggleFlag(b, 0, 0);
    expect(b.cells[0][0].state).toBe('flagged');
    expect(b.flagsPlaced).toBe(1);
    expect(minesRemaining(b)).toBe(1);
    toggleFlag(b, 0, 0);
    expect(b.cells[0][0].state).toBe('hidden');
    expect(b.flagsPlaced).toBe(0);
    expect(minesRemaining(b)).toBe(2);
  });

  it('reveal on a flagged cell is a no-op (R12)', () => {
    const b = boardFromLayout(`
.*
..
`);
    toggleFlag(b, 0, 1); // flag the mine
    reveal(b, 0, 1, noRng);
    expect(b.cells[0][1].state).toBe('flagged');
    expect(b.phase).toBe('playing');
  });

  it('flag on a revealed cell is a no-op', () => {
    const b = boardFromLayout(`
.*.
...
.*.
`);
    reveal(b, 1, 1, noRng);
    expect(b.cells[1][1].state).toBe('revealed');
    toggleFlag(b, 1, 1);
    expect(b.cells[1][1].state).toBe('revealed');
    expect(b.flagsPlaced).toBe(0);
  });

  it('correctly flagging all mines without revealing all safe cells is NOT a win', () => {
    const b = boardFromLayout(`
*..
...
..*
`);
    toggleFlag(b, 0, 0);
    toggleFlag(b, 2, 2);
    expect(minesRemaining(b)).toBe(0);
    expect(b.phase).toBe('playing'); // not won — safe cells still hidden
  });

  it('over-flagging makes mines-remaining go negative (classic behaviour)', () => {
    const b = boardFromLayout(`
*.
..
`);
    toggleFlag(b, 0, 1);
    toggleFlag(b, 1, 0);
    expect(minesRemaining(b)).toBe(-1);
  });
});
