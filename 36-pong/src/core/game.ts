import type { Config, GameState, InputState, Rng } from './types.js';
import { movePlayerPaddle, clampPaddleY } from './paddle.js';
import { computeAiPaddleMove } from './ai.js';
import { stepBall } from './physics.js';
import { serveBall, resetBall } from './serve.js';

/** A new game: ball centred, 0-0, waiting to serve. */
export function createGame(cfg: Config, _rng: Rng): GameState {
  const paddleTop = cfg.table.height / 2 - cfg.paddle.height / 2;
  return {
    ball: {
      pos: { x: cfg.table.width / 2, y: cfg.table.height / 2 },
      vel: { x: 0, y: 0 },
    },
    leftPaddleY: paddleTop,
    rightPaddleY: paddleTop,
    scoreLeft: 0,
    scoreRight: 0,
    phase: 'serving',
    winner: null,
    serveDelay: cfg.serveDelay,
  };
}

/** Advance the game by one timestep, returning a new state. */
export function stepGame(
  state: GameState,
  input: InputState,
  cfg: Config,
  rng: Rng,
  dt: number,
): GameState {
  if (state.phase === 'gameOver') {
    return input.restart ? createGame(cfg, rng) : state;
  }

  // Paddles can move while waiting for the serve too.
  let s = movePlayerPaddle(state, input.playerDir, cfg, dt);
  const aiDir = computeAiPaddleMove(s, cfg, rng);
  const rightY = s.rightPaddleY + aiDir * cfg.ai.speed * dt;
  s = { ...s, rightPaddleY: clampPaddleY(rightY, cfg) };

  if (s.phase === 'serving') {
    const remaining = s.serveDelay - dt;
    return remaining <= 0 ? serveBall(s, cfg, rng) : { ...s, serveDelay: remaining };
  }

  s = stepBall(s, cfg, dt);

  // A point is scored once the ball's centre crosses a goal line.
  if (s.ball.pos.x <= 0) {
    s = { ...s, scoreRight: s.scoreRight + 1 };
  } else if (s.ball.pos.x >= cfg.table.width) {
    s = { ...s, scoreLeft: s.scoreLeft + 1 };
  } else {
    return s;
  }

  if (s.scoreRight >= cfg.winningScore) {
    return { ...s, phase: 'gameOver', winner: 'right' };
  }
  if (s.scoreLeft >= cfg.winningScore) {
    return { ...s, phase: 'gameOver', winner: 'left' };
  }
  return resetBall(s, cfg);
}
