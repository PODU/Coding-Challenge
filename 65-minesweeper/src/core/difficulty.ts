export type DifficultyName = 'beginner' | 'intermediate' | 'expert' | 'custom';

export interface Difficulty {
  rows: number;
  cols: number;
  mines: number;
}

/** Standard Minesweeper presets. */
export const PRESETS: Record<Exclude<DifficultyName, 'custom'>, Difficulty> = {
  beginner: { rows: 9, cols: 9, mines: 10 },
  intermediate: { rows: 16, cols: 16, mines: 40 },
  expert: { rows: 16, cols: 30, mines: 99 },
};

/**
 * Throws if the board size or mine count is invalid. At least one cell must be
 * free of mines, otherwise the first click couldn't be safe.
 */
export function validateDifficulty(d: Difficulty): void {
  if (!Number.isInteger(d.rows) || !Number.isInteger(d.cols) || !Number.isInteger(d.mines)) {
    throw new Error('Difficulty rows, cols and mines must be integers.');
  }
  if (d.rows < 1 || d.cols < 1) {
    throw new Error('Board must have at least 1 row and 1 column.');
  }
  const total = d.rows * d.cols;
  if (d.mines < 1) {
    throw new Error('There must be at least 1 mine.');
  }
  if (d.mines > total - 1) {
    throw new Error(
      `Too many mines: ${d.mines} for a ${d.rows}x${d.cols} board (max ${total - 1}).`,
    );
  }
}
