// Draws a GameState to a canvas. It has no game logic, so tests can run it
// against a mock 2D context.

import type { GameState, PieceId } from '../core/index';
import { COLS, ROWS, COLORS, cellsOf, getGhostRow } from '../core/index';
import { SHAPES } from '../core/index';

type Ctx = CanvasRenderingContext2D;

const BG = '#0b1021';
const GRID_LINE = '#1c2541';
const GHOST = 'rgba(255,255,255,0.18)';

export class CanvasRenderer {
  constructor(
    private readonly cell: number = 30,
    private readonly nextCell: number = 22,
  ) {}

  render(ctx: Ctx, state: GameState): void {
    const w = COLS * this.cell;
    const h = ROWS * this.cell;

    ctx.fillStyle = BG;
    ctx.fillRect(0, 0, w, h);

    // grid lines
    ctx.strokeStyle = GRID_LINE;
    ctx.lineWidth = 1;
    for (let c = 0; c <= COLS; c++) {
      ctx.beginPath();
      ctx.moveTo(c * this.cell + 0.5, 0);
      ctx.lineTo(c * this.cell + 0.5, h);
      ctx.stroke();
    }
    for (let r = 0; r <= ROWS; r++) {
      ctx.beginPath();
      ctx.moveTo(0, r * this.cell + 0.5);
      ctx.lineTo(w, r * this.cell + 0.5);
      ctx.stroke();
    }

    // locked cells
    for (let r = 0; r < ROWS; r++) {
      for (let c = 0; c < COLS; c++) {
        const cell = state.grid[r][c];
        if (cell !== 0) this.drawCell(ctx, c, r, COLORS[cell], this.cell);
      }
    }

    // ghost + active
    if (state.active && (state.phase === 'playing' || state.phase === 'paused')) {
      const ghostRow = getGhostRow(state);
      const ghost = { ...state.active, row: ghostRow };
      for (const { r, c } of cellsOf(ghost)) {
        if (r >= 0) this.drawCell(ctx, c, r, GHOST, this.cell, true);
      }
      for (const { r, c } of cellsOf(state.active)) {
        if (r >= 0) this.drawCell(ctx, c, r, COLORS[state.active.id], this.cell);
      }
    }
  }

  /** Draw the next piece centred in the preview canvas. */
  renderNext(ctx: Ctx, next: PieceId | undefined, boxCols = 4, boxRows = 4): void {
    const w = boxCols * this.nextCell;
    const h = boxRows * this.nextCell;
    ctx.fillStyle = BG;
    ctx.fillRect(0, 0, w, h);
    if (!next) return;

    const offsets = SHAPES[next][0];
    const minC = Math.min(...offsets.map((o) => o.c));
    const maxC = Math.max(...offsets.map((o) => o.c));
    const minR = Math.min(...offsets.map((o) => o.r));
    const maxR = Math.max(...offsets.map((o) => o.r));
    const pieceW = maxC - minC + 1;
    const pieceH = maxR - minR + 1;
    const padC = (boxCols - pieceW) / 2;
    const padR = (boxRows - pieceH) / 2;

    for (const { r, c } of offsets) {
      this.drawCell(ctx, c - minC + padC, r - minR + padR, COLORS[next], this.nextCell);
    }
  }

  private drawCell(
    ctx: Ctx,
    col: number,
    row: number,
    color: string,
    size: number,
    flat = false,
  ): void {
    const x = col * size;
    const y = row * size;
    ctx.fillStyle = color;
    ctx.fillRect(x + 1, y + 1, size - 2, size - 2);
    if (!flat) {
      ctx.strokeStyle = 'rgba(255,255,255,0.25)';
      ctx.lineWidth = 1;
      ctx.strokeRect(x + 1.5, y + 1.5, size - 3, size - 3);
    }
  }
}
