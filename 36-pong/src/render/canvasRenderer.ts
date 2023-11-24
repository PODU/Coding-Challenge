import type { Config, GameState } from '../core/types.js';

export interface Theme {
  bg: string;
  fg: string;
  net: string;
}

export const defaultTheme: Theme = {
  bg: '#0b0b0b',
  fg: '#e8e8e8',
  net: '#4a4a4a',
};

/** Draw the game, scaling table units to the canvas size. */
export function render(
  ctx: CanvasRenderingContext2D,
  state: GameState,
  cfg: Config,
  theme: Theme = defaultTheme,
): void {
  const canvas = ctx.canvas;
  const sx = canvas.width / cfg.table.width;
  const sy = canvas.height / cfg.table.height;

  ctx.fillStyle = theme.bg;
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  // Net
  ctx.strokeStyle = theme.net;
  ctx.lineWidth = Math.max(2, 4 * sx);
  ctx.setLineDash([12 * sy, 14 * sy]);
  ctx.beginPath();
  ctx.moveTo(canvas.width / 2, 0);
  ctx.lineTo(canvas.width / 2, canvas.height);
  ctx.stroke();
  ctx.setLineDash([]);

  // Scores
  ctx.fillStyle = theme.fg;
  ctx.font = `${Math.round(48 * sy)}px "Courier New", monospace`;
  ctx.textBaseline = 'top';
  ctx.textAlign = 'center';
  ctx.fillText(String(state.scoreLeft), canvas.width * 0.25, 16 * sy);
  ctx.fillText(String(state.scoreRight), canvas.width * 0.75, 16 * sy);

  // Paddles
  ctx.fillStyle = theme.fg;
  ctx.fillRect(0, state.leftPaddleY * sy, cfg.paddle.width * sx, cfg.paddle.height * sy);
  ctx.fillRect(
    (cfg.table.width - cfg.paddle.width) * sx,
    state.rightPaddleY * sy,
    cfg.paddle.width * sx,
    cfg.paddle.height * sy,
  );

  // Ball
  ctx.beginPath();
  ctx.arc(
    state.ball.pos.x * sx,
    state.ball.pos.y * sy,
    cfg.ball.radius * ((sx + sy) / 2),
    0,
    Math.PI * 2,
  );
  ctx.fill();

  ctx.textAlign = 'center';
  if (state.phase === 'serving' && state.serveDelay > 0) {
    ctx.font = `${Math.round(22 * sy)}px "Courier New", monospace`;
    ctx.fillText('get ready…', canvas.width / 2, canvas.height * 0.62);
  }
  if (state.phase === 'gameOver') {
    const who = state.winner === 'left' ? 'Player' : 'Computer';
    ctx.font = `${Math.round(40 * sy)}px "Courier New", monospace`;
    ctx.fillText(`${who} wins!`, canvas.width / 2, canvas.height * 0.40);
    ctx.font = `${Math.round(20 * sy)}px "Courier New", monospace`;
    ctx.fillText('press Space / Enter to play again', canvas.width / 2, canvas.height * 0.52);
  }
}
