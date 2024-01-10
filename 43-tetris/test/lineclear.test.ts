import { describe, it, expect } from 'vitest';
import { fullRows, clearRows, emptyGrid } from '../src/core/index';
import type { Grid } from '../src/core/index';
import { fullRow } from './helpers';

describe('line clearing', () => {
  it('detects and clears a single full row, compacting above', () => {
    const g = emptyGrid();
    g[19] = fullRow();
    g[18][0] = 'T'; // a single block to fall down one row

    expect(fullRows(g)).toEqual([19]);
    const cleared = clearRows(g, [19]);
    expect(cleared).toHaveLength(20);
    expect(cleared[19][0]).toBe('T'); // fell from row 18 to 19
    expect(cleared[0].every((c) => c === 0)).toBe(true); // empty row added on top
  });

  it('clears non-contiguous full rows and preserves the order between them', () => {
    const g = emptyGrid();
    g[19] = fullRow('I');
    g[17] = fullRow('I');
    g[18][3] = 'Z'; // partial row sandwiched between two full rows

    expect(fullRows(g)).toEqual([17, 19]);
    const cleared = clearRows(g, [17, 19]);
    // Two empty rows added on top; the partial row survives at the bottom.
    expect(cleared[0].every((c) => c === 0)).toBe(true);
    expect(cleared[1].every((c) => c === 0)).toBe(true);
    expect(cleared[19][3]).toBe('Z');
  });

  it('clears a Tetris (4 rows) together', () => {
    let g: Grid = emptyGrid();
    for (const r of [16, 17, 18, 19]) g[r] = fullRow('I');
    g[15][5] = 'O';

    expect(fullRows(g)).toEqual([16, 17, 18, 19]);
    g = clearRows(g, [16, 17, 18, 19]);
    expect(g[19][5]).toBe('O'); // the lone block fell to the bottom
    for (let r = 0; r < 19; r++) expect(g[r].every((c) => c === 0)).toBe(true);
  });

  it('leaves partial rows untouched', () => {
    const g = emptyGrid();
    g[19][0] = 'L';
    expect(fullRows(g)).toEqual([]);
    expect(clearRows(g, [])).toEqual(g);
  });
});
