import { describe, it, expect } from 'vitest';
import { tryRotate, emptyGrid } from '../src/core/index';
import type { ActivePiece, Grid } from '../src/core/index';

function rotate(grid: Grid, piece: ActivePiece, dir: -1 | 1, simple = false) {
  return tryRotate(grid, piece, dir, simple);
}

describe('rotation & wall kicks', () => {
  it('four CW rotations return to the start (open space)', () => {
    let p: ActivePiece | null = { id: 'T', rotation: 0, row: 8, col: 4 };
    const start = { ...p };
    for (let i = 0; i < 4; i++) p = rotate(emptyGrid(), p!, 1);
    expect(p).toEqual(start);
  });

  it('four CCW rotations return to the start (open space)', () => {
    let p: ActivePiece | null = { id: 'J', rotation: 0, row: 8, col: 4 };
    const start = { ...p };
    for (let i = 0; i < 4; i++) p = rotate(emptyGrid(), p!, -1);
    expect(p).toEqual(start);
  });

  it('O piece rotation is a no-op (no drift)', () => {
    const p: ActivePiece = { id: 'O', rotation: 0, row: 5, col: 4 };
    const r = rotate(emptyGrid(), p, 1)!;
    expect(r.rotation).toBe(0);
    expect(r.row).toBe(5);
    expect(r.col).toBe(4);
  });

  it('kicks the I piece off the left wall (SRS succeeds, simple rejects)', () => {
    // Vertical I against the left wall; rotating to flat would push col -1 out.
    const p: ActivePiece = { id: 'I', rotation: 3, row: 8, col: -1 };
    const srs = rotate(emptyGrid(), p, 1)!;
    expect(srs).not.toBeNull();
    expect(srs.rotation).toBe(0);
    expect(srs.col).toBe(0); // kicked right by one
    expect(rotate(emptyGrid(), p, 1, true)).toBeNull();
  });

  it('kicks a T piece off the floor (SRS succeeds, simple rejects)', () => {
    const p: ActivePiece = { id: 'T', rotation: 0, row: 18, col: 3 };
    const srs = rotate(emptyGrid(), p, 1)!;
    expect(srs).not.toBeNull();
    expect(srs.row).toBeLessThan(18); // lifted up by the floor kick
    expect(rotate(emptyGrid(), p, 1, true)).toBeNull();
  });

  it('rejects rotation when no kick fits (state unchanged)', () => {
    // Everything filled except column 0; a vertical I there cannot go flat.
    const grid: Grid = emptyGrid().map((row) => row.map((_, c) => (c === 0 ? 0 : ('Z' as const))));
    const p: ActivePiece = { id: 'I', rotation: 3, row: 5, col: -1 };
    expect(rotate(grid, p, 1)).toBeNull();
  });
});
