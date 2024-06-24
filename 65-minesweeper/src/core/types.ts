export type CellState = 'hidden' | 'revealed' | 'flagged';

export interface Cell {
  /** Only meaningful once mines are placed. */
  isMine: boolean;
  state: CellState;
  /** Neighbouring mines. Only meaningful once mines are placed. */
  adjacent: number;
}

export type Phase = 'ready' | 'playing' | 'won' | 'lost';

export interface Board {
  rows: number;
  cols: number;
  mines: number;
  /** False until the first reveal. */
  minesPlaced: boolean;
  /** cells[row][col] */
  cells: Cell[][];
  phase: Phase;
  flagsPlaced: number;
}
