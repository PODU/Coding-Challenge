import type { Config, GameState } from './types.js';

export function clamp(value: number, min: number, max: number): number {
  if (value < min) return min;
  if (value > max) return max;
  return value;
}

/** Keep a paddle (given by its top edge) fully on the table. */
export function clampPaddleY(y: number, cfg: Config): number {
  return clamp(y, 0, cfg.table.height - cfg.paddle.height);
}

/** Move the player's (left) paddle; dir is -1 up, 0 none, 1 down. */
export function movePlayerPaddle(
  state: GameState,
  dir: -1 | 0 | 1,
  cfg: Config,
  dt: number,
): GameState {
  const next = state.leftPaddleY + dir * cfg.paddle.speed * dt;
  return { ...state, leftPaddleY: clampPaddleY(next, cfg) };
}
