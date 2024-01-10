import { createGame, reduce, emptyGrid } from '../src/core/index';
import type { ActivePiece, GameState, Grid, PieceId } from '../src/core/index';

/** Start a fresh game (welcome -> playing) deterministically. */
export function startGame(seed = 1, simpleRotation = false): GameState {
  return reduce(createGame({ seed, simpleRotation }), { type: 'START' });
}

/** Replace the active piece (handy for deterministic placement in tests). */
export function withActive(state: GameState, active: ActivePiece): GameState {
  return { ...state, active };
}

/** Replace the grid. */
export function withGrid(state: GameState, grid: Grid): GameState {
  return { ...state, grid };
}

/** A grid where the given (row, col) coordinates are filled with `fill`. */
export function gridWith(cells: Array<[number, number]>, fill: PieceId = 'I'): Grid {
  const g = emptyGrid();
  for (const [r, c] of cells) g[r][c] = fill;
  return g;
}

/** Fill an entire row. */
export function fullRow(fill: PieceId = 'I'): PieceId[] {
  return new Array(10).fill(fill);
}
