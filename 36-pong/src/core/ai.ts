import type { Config, GameState, Rng } from './types.js';

/**
 * Which way the AI (right) paddle moves this step: -1 up, 0 hold, 1 down.
 *
 * It simply chases the ball's height. The dead zone stops it twitching, it is
 * slower than the player's paddle, and cfg.ai.error can add some aim jitter,
 * which together leave it beatable.
 */
export function computeAiPaddleMove(
  state: GameState,
  cfg: Config,
  rng: Rng,
): -1 | 0 | 1 {
  const paddleCenter = state.rightPaddleY + cfg.paddle.height / 2;
  const jitter = cfg.ai.error > 0 ? (rng() * 2 - 1) * cfg.ai.error : 0;
  const diff = state.ball.pos.y + jitter - paddleCenter;

  if (Math.abs(diff) <= cfg.ai.deadZone) return 0;
  return diff > 0 ? 1 : -1;
}
