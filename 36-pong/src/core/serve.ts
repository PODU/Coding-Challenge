import type { Config, GameState, Rng, Side } from './types.js';

/**
 * Launch the ball from the centre toward a random side. The angle is at most
 * maxBounceAngle from horizontal, so the ball never goes straight up or down.
 */
export function serveBall(state: GameState, cfg: Config, rng: Rng): GameState {
  const side: Side = rng() < 0.5 ? 'left' : 'right';
  const angle = (rng() * 2 - 1) * cfg.ball.maxBounceAngle;
  const speed = cfg.ball.baseSpeed;
  const dirX = side === 'right' ? 1 : -1;

  return {
    ...state,
    ball: {
      pos: { x: cfg.table.width / 2, y: cfg.table.height / 2 },
      vel: {
        x: dirX * speed * Math.cos(angle),
        y: speed * Math.sin(angle),
      },
    },
    phase: 'playing',
    serveDelay: 0,
  };
}

/** Put the ball back in the centre and wait serveDelay before serving. */
export function resetBall(state: GameState, cfg: Config): GameState {
  return {
    ...state,
    ball: {
      pos: { x: cfg.table.width / 2, y: cfg.table.height / 2 },
      vel: { x: 0, y: 0 },
    },
    phase: 'serving',
    serveDelay: cfg.serveDelay,
  };
}
