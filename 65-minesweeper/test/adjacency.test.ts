import { describe, it, expect } from 'vitest';
import { boardFromLayout } from './helpers';

describe('adjacency counts', () => {
  it('counts neighbouring mines for interior, edge and corner cells', () => {
    const b = boardFromLayout(`
*..
...
..*
`);
    const adj = (r: number, c: number) => b.cells[r][c].adjacent;
    // corner next to top-left mine
    expect(adj(0, 1)).toBe(1);
    expect(adj(1, 0)).toBe(1);
    // centre touches both diagonal mines
    expect(adj(1, 1)).toBe(2);
    // edges near bottom-right mine
    expect(adj(1, 2)).toBe(1);
    expect(adj(2, 1)).toBe(1);
    // far corners away from mines
    expect(adj(0, 2)).toBe(0);
    expect(adj(2, 0)).toBe(0);
  });

  it('a cell fully surrounded by mines reports 8', () => {
    const b = boardFromLayout(`
***
*.*
***
`);
    expect(b.cells[1][1].adjacent).toBe(8);
  });

  it('mine cells themselves have adjacent 0', () => {
    const b = boardFromLayout(`
*.
..
`);
    expect(b.cells[0][0].isMine).toBe(true);
    expect(b.cells[0][0].adjacent).toBe(0);
  });

  it('edge cell counts only in-bounds neighbours', () => {
    const b = boardFromLayout(`
.*.
.*.
.*.
`);
    // left edge middle cell sees 3 mines in the centre column
    expect(b.cells[1][0].adjacent).toBe(3);
    // top centre is a mine; cell to its right sees 2 mines (top & middle centre)
    expect(b.cells[0][2].adjacent).toBe(2);
  });
});
