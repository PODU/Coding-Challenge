// Grid operations. None of them mutate their input.

import { COLS, ROWS } from './types';
import type { Grid, ActivePiece, Offset } from './types';
import { SHAPES } from './tetrominoes';

export function emptyGrid(): Grid {
  return Array.from({ length: ROWS }, () => new Array<0>(COLS).fill(0));
}

/** Grid cells the piece occupies. */
export function cellsOf(piece: ActivePiece): Offset[] {
  return SHAPES[piece.id][piece.rotation].map((cell) => ({
    r: piece.row + cell.r,
    c: piece.col + cell.c,
  }));
}

/**
 * True if the piece hits a wall, the floor, or a locked cell. Cells above the
 * top (r < 0) are allowed, as in SRS, so kicks can push a piece upward.
 */
export function collides(grid: Grid, piece: ActivePiece): boolean {
  for (const { r, c } of cellsOf(piece)) {
    if (c < 0 || c >= COLS) return true;
    if (r >= ROWS) return true;
    if (r >= 0 && grid[r][c] !== 0) return true;
  }
  return false;
}

export function lock(grid: Grid, piece: ActivePiece): Grid {
  const next = grid.map((row) => row.slice());
  for (const { r, c } of cellsOf(piece)) {
    if (r >= 0 && r < ROWS && c >= 0 && c < COLS) {
      next[r][c] = piece.id;
    }
  }
  return next;
}

export function fullRows(grid: Grid): number[] {
  const rows: number[] = [];
  for (let r = 0; r < ROWS; r++) {
    if (grid[r].every((cell) => cell !== 0)) rows.push(r);
  }
  return rows;
}

/**
 * Removes the given rows (they need not be adjacent) and adds empty rows on
 * top so the grid keeps its size.
 */
export function clearRows(grid: Grid, rows: number[]): Grid {
  if (rows.length === 0) return grid.map((row) => row.slice());
  const remove = new Set(rows);
  const kept = grid.filter((_, r) => !remove.has(r)).map((row) => row.slice());
  const removed = ROWS - kept.length;
  const empty: Grid = Array.from({ length: removed }, () => new Array<0>(COLS).fill(0));
  return [...empty, ...kept];
}
