// Tetromino shapes and spawn positions.
//
// All four rotation states are written out as {r,c} offsets in the piece's
// bounding box (4x4 for I, 2x2 for O, 3x3 otherwise), matching SRS exactly.

import type { PieceId, Offset } from './types';

export const PIECES: PieceId[] = ['I', 'O', 'T', 'S', 'Z', 'J', 'L'];

const o = (r: number, c: number): Offset => ({ r, c });

export const SHAPES: Record<PieceId, Offset[][]> = {
  I: [
    [o(1, 0), o(1, 1), o(1, 2), o(1, 3)],
    [o(0, 2), o(1, 2), o(2, 2), o(3, 2)],
    [o(2, 0), o(2, 1), o(2, 2), o(2, 3)],
    [o(0, 1), o(1, 1), o(2, 1), o(3, 1)],
  ],
  O: [
    [o(0, 0), o(0, 1), o(1, 0), o(1, 1)],
    [o(0, 0), o(0, 1), o(1, 0), o(1, 1)],
    [o(0, 0), o(0, 1), o(1, 0), o(1, 1)],
    [o(0, 0), o(0, 1), o(1, 0), o(1, 1)],
  ],
  T: [
    [o(0, 1), o(1, 0), o(1, 1), o(1, 2)],
    [o(0, 1), o(1, 1), o(1, 2), o(2, 1)],
    [o(1, 0), o(1, 1), o(1, 2), o(2, 1)],
    [o(0, 1), o(1, 0), o(1, 1), o(2, 1)],
  ],
  S: [
    [o(0, 1), o(0, 2), o(1, 0), o(1, 1)],
    [o(0, 1), o(1, 1), o(1, 2), o(2, 2)],
    [o(1, 1), o(1, 2), o(2, 0), o(2, 1)],
    [o(0, 0), o(1, 0), o(1, 1), o(2, 1)],
  ],
  Z: [
    [o(0, 0), o(0, 1), o(1, 1), o(1, 2)],
    [o(0, 2), o(1, 1), o(1, 2), o(2, 1)],
    [o(1, 0), o(1, 1), o(2, 1), o(2, 2)],
    [o(0, 1), o(1, 0), o(1, 1), o(2, 0)],
  ],
  J: [
    [o(0, 0), o(1, 0), o(1, 1), o(1, 2)],
    [o(0, 1), o(0, 2), o(1, 1), o(2, 1)],
    [o(1, 0), o(1, 1), o(1, 2), o(2, 2)],
    [o(0, 1), o(1, 1), o(2, 0), o(2, 1)],
  ],
  L: [
    [o(0, 2), o(1, 0), o(1, 1), o(1, 2)],
    [o(0, 1), o(1, 1), o(2, 1), o(2, 2)],
    [o(1, 0), o(1, 1), o(1, 2), o(2, 0)],
    [o(0, 0), o(0, 1), o(1, 1), o(2, 1)],
  ],
};

/** Spawn position (top-left of bounding box) for each piece on a 10-wide well. */
export const SPAWN: Record<PieceId, { row: number; col: number }> = {
  I: { row: 0, col: 3 },
  O: { row: 0, col: 4 },
  T: { row: 0, col: 3 },
  S: { row: 0, col: 3 },
  Z: { row: 0, col: 3 },
  J: { row: 0, col: 3 },
  L: { row: 0, col: 3 },
};

/** Standard tetromino colors (used by the canvas shell). */
export const COLORS: Record<PieceId, string> = {
  I: '#22d3ee',
  O: '#facc15',
  T: '#a855f7',
  S: '#22c55e',
  Z: '#ef4444',
  J: '#3b82f6',
  L: '#f97316',
};
