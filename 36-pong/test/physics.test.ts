import { test } from 'node:test';
import assert from 'node:assert/strict';
import { stepBall } from '../src/core/physics.js';
import { cfg, makeState, mag } from './helpers.js';

const c = cfg();

test('T1 ball moving up reflects off the top wall (vel.y flips, vel.x kept)', () => {
  const s = makeState(c, {
    ball: { pos: { x: 400, y: 12 }, vel: { x: 100, y: -360 } },
  });
  const next = stepBall(s, c, 0.05);
  assert.ok(next.ball.vel.y > 0, 'vertical velocity reversed downward');
  assert.equal(Math.abs(next.ball.vel.y), 360, 'vertical speed preserved');
  assert.equal(next.ball.vel.x, 100, 'horizontal velocity unchanged');
  assert.ok(next.ball.pos.y >= c.ball.radius, 'pushed back inside the field');
});

test('T2 ball moving down reflects off the bottom wall', () => {
  const s = makeState(c, {
    ball: { pos: { x: 400, y: c.table.height - 12 }, vel: { x: -80, y: 360 } },
  });
  const next = stepBall(s, c, 0.05);
  assert.ok(next.ball.vel.y < 0, 'vertical velocity reversed upward');
  assert.equal(next.ball.vel.x, -80, 'horizontal velocity unchanged');
  assert.ok(next.ball.pos.y <= c.table.height - c.ball.radius, 'pushed inside');
});

test('T3 horizontal speed is conserved across a wall bounce', () => {
  const s = makeState(c, {
    ball: { pos: { x: 400, y: 11 }, vel: { x: 222, y: -360 } },
  });
  const next = stepBall(s, c, 0.05);
  assert.equal(next.ball.vel.x, 222);
});

test('T4 free-flight integration: pos += vel*dt exactly when no collision', () => {
  const s = makeState(c, {
    ball: { pos: { x: 400, y: 240 }, vel: { x: 100, y: 50 } },
  });
  const next = stepBall(s, c, 0.1);
  assert.equal(next.ball.pos.x, 410);
  assert.equal(next.ball.pos.y, 245);
  assert.deepEqual(next.ball.vel, { x: 100, y: 50 });
});

test('T5 ball overlapping the left paddle reverses vel.x to positive', () => {
  const s = makeState(c, {
    leftPaddleY: 200,
    ball: { pos: { x: 20, y: 240 }, vel: { x: -200, y: 0 } },
  });
  const next = stepBall(s, c, 0.05);
  assert.ok(next.ball.vel.x > 0, 'ball now travels right');
});

test('T6 ball overlapping the right paddle reverses vel.x to negative', () => {
  const s = makeState(c, {
    rightPaddleY: 200,
    ball: { pos: { x: 780, y: 240 }, vel: { x: 200, y: 0 } },
  });
  const next = stepBall(s, c, 0.05);
  assert.ok(next.ball.vel.x < 0, 'ball now travels left');
});

test('T7 centre hit leaves a near-horizontal angle (small |vel.y|)', () => {
  const center = 200 + c.paddle.height / 2;
  const s = makeState(c, {
    leftPaddleY: 200,
    ball: { pos: { x: 20, y: center }, vel: { x: -300, y: 0 } },
  });
  const next = stepBall(s, c, 0.05);
  assert.ok(next.ball.vel.x > 0);
  assert.ok(Math.abs(next.ball.vel.y) < 1, 'outgoing path nearly horizontal');
});

test('T8 top-edge hit leaves a steep upward angle within ±maxBounceAngle', () => {
  const s = makeState(c, {
    leftPaddleY: 200,
    ball: { pos: { x: 20, y: 200 }, vel: { x: -300, y: 0 } }, // top edge of paddle
  });
  const next = stepBall(s, c, 0.05);
  assert.ok(next.ball.vel.x > 0);
  assert.ok(next.ball.vel.y < 0, 'deflected upward');
  const angle = Math.abs(Math.atan2(next.ball.vel.y, Math.abs(next.ball.vel.x)));
  assert.ok(angle <= c.ball.maxBounceAngle + 1e-9, 'within the max bounce angle');
  assert.ok(angle > 0.5, 'steep, not shallow');
});

test('T9 speed is conserved/clamped on a bounce, never increased past the cap', () => {
  const center = 200 + c.paddle.height / 2;
  // Below base speed gets raised to base; above max gets clamped to max.
  const fast = makeState(c, {
    leftPaddleY: 200,
    ball: { pos: { x: 20, y: center }, vel: { x: -900, y: 0 } }, // mag 900 > maxSpeed 720
  });
  const out = stepBall(fast, c, 0.05).ball.vel;
  assert.ok(mag(out.x, out.y) <= c.ball.maxSpeed + 1e-9);
  assert.ok(Math.abs(mag(out.x, out.y) - c.ball.maxSpeed) < 1e-6, 'clamped to maxSpeed');

  const normal = makeState(c, {
    leftPaddleY: 200,
    ball: { pos: { x: 20, y: center }, vel: { x: -500, y: 0 } },
  });
  const out2 = stepBall(normal, c, 0.05).ball.vel;
  assert.ok(Math.abs(mag(out2.x, out2.y) - 500) < 1e-6, 'preserved, not increased');
});

test('T10 swept check: a single huge step that would tunnel is still a hit', () => {
  const s = makeState(c, {
    leftPaddleY: 200,
    ball: { pos: { x: 200, y: 240 }, vel: { x: -8000, y: 0 } },
  });
  const next = stepBall(s, c, 0.05); // would reach x = -200 without a swept test
  assert.ok(next.ball.vel.x > 0, 'detected and reflected, not tunnelled');
  const leftPlane = c.paddle.width + c.ball.radius;
  assert.ok(Math.abs(next.ball.pos.x - leftPlane) < 1e-9, 'snapped to the contact plane');
});
