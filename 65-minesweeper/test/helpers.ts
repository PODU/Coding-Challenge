/** Shared test helpers for building hand-crafted boards. */

import { createBoard, computeAdjacency } from '../src/core/board';
import type { Board } from '../src/core/types';

/**
 * Builds a board from an ASCII layout where '*' is a mine and any other char is
 * safe. Mines are marked placed and adjacency computed, so it behaves as a
 * post-first-click board. Lines must all be the same width.
 */
export function boardFromLayout(layout: string): Board {
  const rows = layout
    .split('\n')
    .map((l) => l.trimEnd())
    .filter((l) => l.length > 0);
  const r = rows.length;
  const c = rows[0].length;
  const mineCoords: Array<[number, number]> = [];
  for (let i = 0; i < r; i++) {
    if (rows[i].length !== c) throw new Error('Ragged layout');
    for (let j = 0; j < c; j++) {
      if (rows[i][j] === '*') mineCoords.push([i, j]);
    }
  }
  const b = createBoard({ rows: r, cols: c, mines: mineCoords.length });
  for (const [mr, mc] of mineCoords) b.cells[mr][mc].isMine = true;
  b.minesPlaced = true;
  b.phase = 'playing';
  computeAdjacency(b);
  return b;
}

/** Returns the set of mine coordinates as "r,c" strings. */
export function mineCoords(b: Board): Set<string> {
  const out = new Set<string>();
  for (let r = 0; r < b.rows; r++) {
    for (let c = 0; c < b.cols; c++) {
      if (b.cells[r][c].isMine) out.add(`${r},${c}`);
    }
  }
  return out;
}
