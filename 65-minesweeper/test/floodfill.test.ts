import { describe, it, expect } from 'vitest';
import { reveal, revealedCount } from '../src/core/board';
import { mulberry32 } from '../src/core/rng';
import { boardFromLayout } from './helpers';

const noRng = mulberry32(0); // mines already placed in these hand-built boards

describe('flood-fill region', () => {
  it('reveals the connected zero region plus its numbered border, nothing beyond', () => {
    // Mine isolated in the bottom-right; revealing top-left floods most cells.
    const b = boardFromLayout(`
....
....
....
...*
`);
    reveal(b, 0, 0, noRng);
    // Everything except the mine should be revealed (single connected zero area).
    expect(revealedCount(b)).toBe(15);
    expect(b.cells[3][3].state).toBe('hidden'); // the mine stays hidden
  });

  it('stops at numbered cells and does not propagate through them', () => {
    // A column of mines (col 3) splits the board into two zero regions.
    const b = boardFromLayout(`
...*...
...*...
...*...
`);
    reveal(b, 0, 0, noRng);
    // Left flood: cols 0 & 1 are zero; col 2 is the numbered border.
    expect(b.cells[0][0].state).toBe('revealed');
    expect(b.cells[2][0].state).toBe('revealed');
    expect(b.cells[1][1].state).toBe('revealed');
    expect(b.cells[0][2].state).toBe('revealed'); // numbered border revealed
    // The mine wall and the far side must NOT be revealed.
    expect(b.cells[0][3].state).toBe('hidden'); // mine
    expect(b.cells[0][4].state).toBe('hidden');
    expect(b.cells[0][6].state).toBe('hidden');
  });

  it('revealing a numbered cell reveals only that cell', () => {
    const b = boardFromLayout(`
...*...
...*...
...*...
`);
    reveal(b, 0, 2, noRng); // a numbered border cell — no flood
    expect(b.cells[0][2].state).toBe('revealed');
    expect(revealedCount(b)).toBe(1);
  });

  it('flood-fill at a corner stays in bounds and terminates', () => {
    // One mine in the opposite corner; revealing (4,4) floods the rest in bounds.
    const b = boardFromLayout(`
*....
.....
.....
.....
.....
`);
    reveal(b, 4, 4, noRng); // bottom-right corner -> reveals all 24 safe cells
    expect(revealedCount(b)).toBe(24);
    expect(b.cells[0][0].state).toBe('hidden'); // the mine
  });

  it('does not reveal flagged cells during flood-fill', () => {
    const b = boardFromLayout(`
....
....
....
...*
`);
    // Flag a cell that the flood would otherwise reach.
    b.cells[0][1].state = 'flagged';
    b.flagsPlaced = 1;
    reveal(b, 0, 0, noRng);
    expect(b.cells[0][1].state).toBe('flagged');
  });
});
