import { alienRect } from '../core/fleet.js';
import type { GameState } from '../core/types.js';

const COLORS = {
  bg: '#05060a',
  field: '#0b0f1a',
  ship: '#5ce66b',
  shot: '#ffffff',
  bomb: '#ff5d5d',
  bunker: '#39d353',
  ufo: '#ff4fd8',
  text: '#e6edf3',
  alienRows: ['#ff6b6b', '#ffd166', '#ffd166', '#6bd0ff', '#6bd0ff'],
};

/** Draw the whole frame from the game state. */
export function draw(ctx: CanvasRenderingContext2D, state: GameState): void {
  const { bounds } = state;

  ctx.fillStyle = COLORS.bg;
  ctx.fillRect(0, 0, bounds.w, bounds.h);
  ctx.fillStyle = COLORS.field;
  ctx.fillRect(bounds.x, bounds.y, bounds.w, bounds.h);

  const f = state.fleet;
  for (let r = 0; r < f.rows; r++) {
    ctx.fillStyle = COLORS.alienRows[r % COLORS.alienRows.length];
    for (let c = 0; c < f.cols; c++) {
      if (!f.alive[r][c]) continue;
      const a = alienRect(f, r, c);
      ctx.fillRect(a.x, a.y, a.w, a.h);
      // eyes
      ctx.fillStyle = COLORS.bg;
      ctx.fillRect(a.x + 4, a.y + a.h - 4, 3, 3);
      ctx.fillRect(a.x + a.w - 7, a.y + a.h - 4, 3, 3);
      ctx.fillStyle = COLORS.alienRows[r % COLORS.alienRows.length];
    }
  }

  ctx.fillStyle = COLORS.bunker;
  for (const b of state.bunkers) {
    for (let r = 0; r < b.rows; r++) {
      for (let c = 0; c < b.cols; c++) {
        if (!b.cells[r][c]) continue;
        ctx.fillRect(
          b.pos.x + c * b.cellSize.x,
          b.pos.y + r * b.cellSize.y,
          b.cellSize.x,
          b.cellSize.y,
        );
      }
    }
  }

  if (state.ufo) {
    ctx.fillStyle = COLORS.ufo;
    ctx.fillRect(state.ufo.pos.x, state.ufo.pos.y, state.ufo.w, state.ufo.h);
  }

  const s = state.ship;
  ctx.fillStyle = COLORS.ship;
  ctx.fillRect(s.pos.x, s.pos.y + 4, s.w, s.h - 4);
  ctx.fillRect(s.pos.x + s.w / 2 - 2, s.pos.y, 4, 6); // cannon

  if (state.playerShot) {
    ctx.fillStyle = COLORS.shot;
    const p = state.playerShot;
    ctx.fillRect(p.pos.x, p.pos.y, p.w, p.h);
  }

  ctx.fillStyle = COLORS.bomb;
  for (const b of state.bombs) ctx.fillRect(b.pos.x, b.pos.y, b.w, b.h);

  ctx.fillStyle = COLORS.text;
  ctx.font = '16px monospace';
  ctx.textBaseline = 'top';
  ctx.fillText(`SCORE ${state.score}`, bounds.x + 12, bounds.y + 8);
  ctx.fillText(`WAVE ${state.wave}`, bounds.x + bounds.w / 2 - 30, bounds.y + 8);
  ctx.textAlign = 'right';
  ctx.fillText(`LIVES ${state.ship.lives}`, bounds.x + bounds.w - 12, bounds.y + 8);
  ctx.textAlign = 'left';

  if (state.phase === 'gameOver') overlay(ctx, bounds.w, bounds.h, 'GAME OVER', `Final Score ${state.score}  -  Press Enter`);
  else if (state.phase === 'ready') overlay(ctx, bounds.w, bounds.h, 'SPACE INVADERS', 'Press Enter to start');
}

function overlay(
  ctx: CanvasRenderingContext2D,
  w: number,
  h: number,
  title: string,
  subtitle: string,
): void {
  ctx.fillStyle = 'rgba(0,0,0,0.6)';
  ctx.fillRect(0, 0, w, h);
  ctx.fillStyle = COLORS.text;
  ctx.textAlign = 'center';
  ctx.font = '40px monospace';
  ctx.fillText(title, w / 2, h / 2 - 30);
  ctx.font = '18px monospace';
  ctx.fillText(subtitle, w / 2, h / 2 + 20);
  ctx.textAlign = 'left';
}
