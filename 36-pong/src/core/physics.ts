import type { Config, GameState, Vec2 } from './types.js';
import { clamp } from './paddle.js';

/**
 * Velocity after hitting a paddle. The further from the paddle's centre the
 * ball hits, the steeper it leaves, up to maxBounceAngle at the very edge.
 * Speed is kept, within [baseSpeed, maxSpeed]. dirX is the new horizontal
 * direction: 1 off the left paddle, -1 off the right.
 */
function bounceOffPaddle(
  vel: Vec2,
  ballY: number,
  paddleTopY: number,
  cfg: Config,
  dirX: 1 | -1,
): Vec2 {
  const half = cfg.paddle.height / 2;
  const paddleCenterY = paddleTopY + half;
  const offset = half > 0 ? clamp((ballY - paddleCenterY) / half, -1, 1) : 0;
  const angle = offset * cfg.ball.maxBounceAngle;
  const speed = clamp(Math.hypot(vel.x, vel.y), cfg.ball.baseSpeed, cfg.ball.maxSpeed);
  return {
    x: dirX * speed * Math.cos(angle),
    y: speed * Math.sin(angle),
  };
}

/** Is the ball level with the paddle, counting the ball's radius? */
function withinPaddleY(ballY: number, paddleTopY: number, cfg: Config): boolean {
  const r = cfg.ball.radius;
  return ballY >= paddleTopY - r && ballY <= paddleTopY + cfg.paddle.height + r;
}

/**
 * Move the ball one step, bouncing off the walls and paddles.
 *
 * Walls are handled first so a ball hitting a wall and a paddle in the same
 * step still bounces off the paddle. The paddle check only asks whether the
 * ball has reached or passed the paddle's face, so a big step can't jump
 * through it.
 */
export function stepBall(state: GameState, cfg: Config, dt: number): GameState {
  const r = cfg.ball.radius;
  const { width, height } = cfg.table;
  const pw = cfg.paddle.width;

  let nx = state.ball.pos.x + state.ball.vel.x * dt;
  let ny = state.ball.pos.y + state.ball.vel.y * dt;
  let vel: Vec2 = { x: state.ball.vel.x, y: state.ball.vel.y };

  const topLimit = r;
  const bottomLimit = height - r;
  if (ny < topLimit && vel.y < 0) {
    ny = 2 * topLimit - ny; // mirror the overshoot back onto the table
    vel = { x: vel.x, y: -vel.y };
  } else if (ny > bottomLimit && vel.y > 0) {
    ny = 2 * bottomLimit - ny;
    vel = { x: vel.x, y: -vel.y };
  }

  // A ball that gets behind a paddle is scored by game.ts on the same step, so
  // "moving toward the paddle and at or past its face" can't bounce it twice.
  const leftPlane = pw + r;
  if (vel.x < 0 && nx <= leftPlane && withinPaddleY(ny, state.leftPaddleY, cfg)) {
    nx = leftPlane;
    vel = bounceOffPaddle(vel, ny, state.leftPaddleY, cfg, 1);
  } else {
    const rightPlane = width - pw - r;
    if (vel.x > 0 && nx >= rightPlane && withinPaddleY(ny, state.rightPaddleY, cfg)) {
      nx = rightPlane;
      vel = bounceOffPaddle(vel, ny, state.rightPaddleY, cfg, -1);
    }
  }

  return {
    ...state,
    ball: { pos: { x: nx, y: ny }, vel },
  };
}
