export type PieceId = 'I' | 'O' | 'T' | 'S' | 'Z' | 'J' | 'L';

/** 0 = empty cell, otherwise the piece id that filled it. */
export type Cell = 0 | PieceId;

/** Grid is indexed [row][col]; row 0 is the top. */
export type Grid = Cell[][];

export type Rotation = 0 | 1 | 2 | 3;

/** A cell offset or position. */
export interface Offset {
  r: number;
  c: number;
}

export interface ActivePiece {
  id: PieceId;
  rotation: Rotation;
  /** Row of the top-left of the piece's bounding box. */
  row: number;
  /** Column of the top-left of the piece's bounding box. */
  col: number;
}

export type Phase = 'welcome' | 'playing' | 'paused' | 'gameover';

export interface GameState {
  grid: Grid;
  active: ActivePiece | null;
  /** Upcoming pieces; nextQueue[0] spawns next. */
  nextQueue: PieceId[];
  /** What is left of the current 7-bag. */
  bag: PieceId[];
  rngState: number;
  score: number;
  /** Total lines cleared. */
  lines: number;
  level: number;
  phase: Phase;
  /** Milliseconds per cell of gravity at the current level. */
  gravityMs: number;
  /** Time since the last gravity step. */
  dropAccumulatorMs: number;
  /** Rotate without SRS wall kicks. */
  simpleRotation: boolean;
}

export type Action =
  | { type: 'TICK'; dtMs: number }
  | { type: 'MOVE'; dir: -1 | 1 }
  | { type: 'SOFT_DROP' }
  | { type: 'HARD_DROP' }
  | { type: 'ROTATE'; dir: -1 | 1 }
  | { type: 'PAUSE' }
  | { type: 'RESUME' }
  | { type: 'START' }
  | { type: 'RESTART' };

export const COLS = 10;
export const ROWS = 20;
