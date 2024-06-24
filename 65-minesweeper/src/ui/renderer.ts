import type { Board, Cell } from '../core/types';
import { minesRemaining } from '../core/board';

/** With `revealMines`, hidden mines are shown as well. */
export function cellGlyph(cell: Cell, revealMines = false): string {
  if (cell.state === 'flagged') return 'F';
  if (cell.state === 'hidden') {
    return revealMines && cell.isMine ? '*' : '.';
  }
  if (cell.isMine) return '*';
  return cell.adjacent === 0 ? ' ' : String(cell.adjacent);
}

function pad(n: number, width: number): string {
  return String(n).padStart(width, ' ');
}

/** Draws the grid with row and column numbers. A lost game shows all mines. */
export function renderBoard(b: Board): string {
  const revealMines = b.phase === 'lost';
  const rowLabelWidth = String(b.rows - 1).length;
  const colLabelWidth = String(b.cols - 1).length;

  const lines: string[] = [];

  let header = ' '.repeat(rowLabelWidth + 1);
  for (let c = 0; c < b.cols; c++) {
    header += pad(c, colLabelWidth) + ' ';
  }
  lines.push(header.replace(/\s+$/, ''));

  for (let r = 0; r < b.rows; r++) {
    let line = pad(r, rowLabelWidth) + ' ';
    for (let c = 0; c < b.cols; c++) {
      line += cellGlyph(b.cells[r][c], revealMines).padStart(colLabelWidth, ' ') + ' ';
    }
    lines.push(line.replace(/\s+$/, ''));
  }

  return lines.join('\n');
}

export function renderStatus(b: Board, elapsedSeconds: number): string {
  const phaseLabel: Record<Board['phase'], string> = {
    ready: 'Ready',
    playing: 'Playing',
    won: 'You WON! 🎉',
    lost: 'BOOM! You lost. 💥',
  };
  return `Mines left: ${minesRemaining(b)}   Time: ${elapsedSeconds}s   ${phaseLabel[b.phase]}`;
}
