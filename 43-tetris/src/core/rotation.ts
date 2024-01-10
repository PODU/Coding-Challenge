// Rotation with SRS wall kicks.
//
// The kick tables are the standard SRS ones converted to our coordinates, where
// rows grow downward: dc = x, dr = -y. Keys are "from>to" rotation indices.

import type { ActivePiece, Grid, Rotation } from './types';
import { collides } from './board';

interface Kick {
  dr: number;
  dc: number;
}

const k = (dr: number, dc: number): Kick => ({ dr, dc });

const JLSTZ_KICKS: Record<string, Kick[]> = {
  '0>1': [k(0, 0), k(0, -1), k(-1, -1), k(2, 0), k(2, -1)],
  '1>0': [k(0, 0), k(0, 1), k(1, 1), k(-2, 0), k(-2, 1)],
  '1>2': [k(0, 0), k(0, 1), k(1, 1), k(-2, 0), k(-2, 1)],
  '2>1': [k(0, 0), k(0, -1), k(-1, -1), k(2, 0), k(2, -1)],
  '2>3': [k(0, 0), k(0, 1), k(-1, 1), k(2, 0), k(2, 1)],
  '3>2': [k(0, 0), k(0, -1), k(1, -1), k(-2, 0), k(-2, -1)],
  '3>0': [k(0, 0), k(0, -1), k(1, -1), k(-2, 0), k(-2, -1)],
  '0>3': [k(0, 0), k(0, 1), k(-1, 1), k(2, 0), k(2, 1)],
};

const I_KICKS: Record<string, Kick[]> = {
  '0>1': [k(0, 0), k(0, -2), k(0, 1), k(1, -2), k(-2, 1)],
  '1>0': [k(0, 0), k(0, 2), k(0, -1), k(-1, 2), k(2, -1)],
  '1>2': [k(0, 0), k(0, -1), k(0, 2), k(-2, -1), k(1, 2)],
  '2>1': [k(0, 0), k(0, 1), k(0, -2), k(2, 1), k(-1, -2)],
  '2>3': [k(0, 0), k(0, 2), k(0, -1), k(-1, 2), k(2, -1)],
  '3>2': [k(0, 0), k(0, -2), k(0, 1), k(1, -2), k(-2, 1)],
  '3>0': [k(0, 0), k(0, 1), k(0, -2), k(2, 1), k(-1, -2)],
  '0>3': [k(0, 0), k(0, -1), k(0, 2), k(-2, -1), k(1, 2)],
};

/**
 * Returns the rotated (and possibly kicked) piece, or null if it doesn't fit.
 * With `simple` set, no kicks are tried. O never changes.
 */
export function tryRotate(
  grid: Grid,
  piece: ActivePiece,
  dir: -1 | 1,
  simple: boolean,
): ActivePiece | null {
  if (piece.id === 'O') return { ...piece };

  const from = piece.rotation;
  const to = (((from + dir) % 4) + 4) % 4 as Rotation;
  const rotated: ActivePiece = { ...piece, rotation: to };

  if (simple) {
    return collides(grid, rotated) ? null : rotated;
  }

  const table = piece.id === 'I' ? I_KICKS : JLSTZ_KICKS;
  const kicks = table[`${from}>${to}`];
  for (const kick of kicks) {
    const candidate: ActivePiece = {
      ...rotated,
      row: rotated.row + kick.dr,
      col: rotated.col + kick.dc,
    };
    if (!collides(grid, candidate)) return candidate;
  }
  return null;
}
