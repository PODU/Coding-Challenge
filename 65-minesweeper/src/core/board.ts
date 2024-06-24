/**
 * The Minesweeper rules. No I/O; functions mutate the board in place and
 * return it. Randomness comes from the RNG passed in, so a seeded RNG always
 * gives the same game.
 */

import type { Board, Cell } from './types';
import type { Difficulty } from './difficulty';
import { validateDifficulty } from './difficulty';
import type { RNG } from './rng';

const NEIGHBOURS: ReadonlyArray<readonly [number, number]> = [
  [-1, -1], [-1, 0], [-1, 1],
  [0, -1], [0, 1],
  [1, -1], [1, 0], [1, 1],
];

export function inBounds(b: Board, r: number, c: number): boolean {
  return r >= 0 && r < b.rows && c >= 0 && c < b.cols;
}

function neighbours(b: Board, r: number, c: number): Array<[number, number]> {
  const out: Array<[number, number]> = [];
  for (const [dr, dc] of NEIGHBOURS) {
    if (inBounds(b, r + dr, c + dc)) out.push([r + dr, c + dc]);
  }
  return out;
}

/** Mines aren't placed until the first reveal, so the first click is safe. */
export function createBoard(d: Difficulty): Board {
  validateDifficulty(d);
  const cells: Cell[][] = Array.from({ length: d.rows }, () =>
    Array.from({ length: d.cols }, (): Cell => ({ isMine: false, state: 'hidden', adjacent: 0 })),
  );
  return {
    rows: d.rows,
    cols: d.cols,
    mines: d.mines,
    minesPlaced: false,
    cells,
    phase: 'ready',
    flagsPlaced: 0,
  };
}

/**
 * Places the mines anywhere except the first-clicked cell. Its neighbours are
 * kept clear too, so the first click opens an area, unless the board is too
 * crowded for that.
 */
export function placeMines(b: Board, rng: RNG, safeR: number, safeC: number): Board {
  if (b.minesPlaced) return b;

  const total = b.rows * b.cols;
  const safeIndex = safeR * b.cols + safeC;

  const excluded = new Set<number>([safeIndex]);
  for (const [nr, nc] of neighbours(b, safeR, safeC)) {
    excluded.add(nr * b.cols + nc);
  }
  if (total - excluded.size < b.mines) {
    excluded.clear();
    excluded.add(safeIndex);
  }

  const candidates: number[] = [];
  for (let i = 0; i < total; i++) {
    if (!excluded.has(i)) candidates.push(i);
  }

  // Partial Fisher-Yates shuffle: the first `mines` candidates get the mines.
  for (let i = 0; i < b.mines; i++) {
    const j = i + rng.nextInt(candidates.length - i);
    [candidates[i], candidates[j]] = [candidates[j], candidates[i]];
    const idx = candidates[i];
    b.cells[Math.floor(idx / b.cols)][idx % b.cols].isMine = true;
  }

  b.minesPlaced = true;
  computeAdjacency(b);
  return b;
}

export function computeAdjacency(b: Board): Board {
  for (let r = 0; r < b.rows; r++) {
    for (let c = 0; c < b.cols; c++) {
      const cell = b.cells[r][c];
      if (cell.isMine) {
        cell.adjacent = 0;
        continue;
      }
      let count = 0;
      for (const [nr, nc] of neighbours(b, r, c)) {
        if (b.cells[nr][nc].isMine) count++;
      }
      cell.adjacent = count;
    }
  }
  return b;
}

/** Used when the game is lost. */
export function revealAllMines(b: Board): Board {
  for (const row of b.cells) {
    for (const cell of row) {
      if (cell.isMine) cell.state = 'revealed';
    }
  }
  return b;
}

/**
 * Reveals (r, c). Illegal moves (game over, out of bounds, already revealed or
 * flagged) are ignored. The first reveal places the mines. A cell with no
 * neighbouring mines opens up the whole empty region around it.
 */
export function reveal(b: Board, r: number, c: number, rng: RNG): Board {
  if (b.phase === 'won' || b.phase === 'lost') return b;
  if (!inBounds(b, r, c)) return b;

  const cell = b.cells[r][c];
  if (cell.state !== 'hidden') return b;

  if (!b.minesPlaced) {
    placeMines(b, rng, r, c);
    b.phase = 'playing';
  }

  if (cell.isMine) {
    cell.state = 'revealed';
    b.phase = 'lost';
    revealAllMines(b);
    return b;
  }

  floodReveal(b, r, c);
  evaluateStatus(b);
  return b;
}

/**
 * Reveals (r, c) and, if it is a zero, spreads to its neighbours. Numbered
 * cells are revealed but don't spread further, and flags are left alone.
 */
function floodReveal(b: Board, r: number, c: number): void {
  const stack: Array<[number, number]> = [[r, c]];
  while (stack.length > 0) {
    const [cr, cc] = stack.pop()!;
    const cell = b.cells[cr][cc];
    if (cell.state !== 'hidden') continue;
    cell.state = 'revealed';
    if (cell.adjacent === 0) {
      for (const [nr, nc] of neighbours(b, cr, cc)) {
        const ncell = b.cells[nr][nc];
        if (ncell.state === 'hidden' && !ncell.isMine) {
          stack.push([nr, nc]);
        }
      }
    }
  }
}

/** Flags or unflags a hidden cell. Ignored on revealed cells and after the game ends. */
export function toggleFlag(b: Board, r: number, c: number): Board {
  if (b.phase === 'won' || b.phase === 'lost') return b;
  if (!inBounds(b, r, c)) return b;

  const cell = b.cells[r][c];
  if (cell.state === 'hidden') {
    cell.state = 'flagged';
    b.flagsPlaced++;
  } else if (cell.state === 'flagged') {
    cell.state = 'hidden';
    b.flagsPlaced--;
  }
  return b;
}

/** Sets the phase to 'won' once every safe cell is revealed, or 'lost' if a mine is. */
export function evaluateStatus(b: Board): Board {
  let mineRevealed = false;
  let revealedSafe = 0;
  for (const row of b.cells) {
    for (const cell of row) {
      if (cell.state === 'revealed') {
        if (cell.isMine) mineRevealed = true;
        else revealedSafe++;
      }
    }
  }
  if (mineRevealed) {
    b.phase = 'lost';
  } else if (revealedSafe === b.rows * b.cols - b.mines) {
    b.phase = 'won';
  }
  return b;
}

/** Mines minus flags placed. Goes negative if the player over-flags. */
export function minesRemaining(b: Board): number {
  return b.mines - b.flagsPlaced;
}

export function isOver(b: Board): boolean {
  return b.phase === 'won' || b.phase === 'lost';
}

export function revealedCount(b: Board): number {
  let n = 0;
  for (const row of b.cells) {
    for (const cell of row) {
      if (cell.state === 'revealed') n++;
    }
  }
  return n;
}
