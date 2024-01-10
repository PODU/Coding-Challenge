import { describe, it, expect } from 'vitest';
import { collides, emptyGrid } from '../src/core/index';
import type { ActivePiece } from '../src/core/index';
import { gridWith } from './helpers';

describe('collision detection', () => {
  it('returns false for a piece in open space', () => {
    const piece: ActivePiece = { id: 'T', rotation: 0, row: 5, col: 4 };
    expect(collides(emptyGrid(), piece)).toBe(false);
  });

  it('detects the left wall', () => {
    // T rotation 0 has its leftmost cell at offset col 0 in row 1.
    const piece: ActivePiece = { id: 'T', rotation: 0, row: 5, col: -1 };
    expect(collides(emptyGrid(), piece)).toBe(true);
  });

  it('detects the right wall', () => {
    const piece: ActivePiece = { id: 'T', rotation: 0, row: 5, col: 8 };
    expect(collides(emptyGrid(), piece)).toBe(true);
  });

  it('detects the floor', () => {
    const piece: ActivePiece = { id: 'O', rotation: 0, row: 19, col: 4 };
    expect(collides(emptyGrid(), piece)).toBe(true);
  });

  it('allows cells above the top (buffer zone)', () => {
    const piece: ActivePiece = { id: 'I', rotation: 1, row: -2, col: 3 };
    expect(collides(emptyGrid(), piece)).toBe(false);
  });

  it('detects overlap with locked cells', () => {
    const grid = gridWith([[6, 4]]);
    const overlapping: ActivePiece = { id: 'O', rotation: 0, row: 5, col: 4 };
    expect(collides(grid, overlapping)).toBe(true);
    const clear: ActivePiece = { id: 'O', rotation: 0, row: 5, col: 0 };
    expect(collides(grid, clear)).toBe(false);
  });

  it('I piece can reach both walls horizontally', () => {
    expect(collides(emptyGrid(), { id: 'I', rotation: 0, row: 5, col: 0 })).toBe(false);
    expect(collides(emptyGrid(), { id: 'I', rotation: 0, row: 5, col: 6 })).toBe(false);
    expect(collides(emptyGrid(), { id: 'I', rotation: 0, row: 5, col: 7 })).toBe(true);
    expect(collides(emptyGrid(), { id: 'I', rotation: 0, row: 5, col: -1 })).toBe(true);
  });
});
