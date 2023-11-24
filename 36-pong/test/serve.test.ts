import { test } from 'node:test';
import assert from 'node:assert/strict';
import { serveBall } from '../src/core/serve.js';
import { cfg, makeState, seqRng, mag } from './helpers.js';

const c = cfg();

test('T17 after serve the ball sits on the centre line', () => {
  const s = serveBall(makeState(c, { phase: 'serving' }), c, seqRng([0.2, 0.6]));
  assert.equal(s.ball.pos.x, c.table.width / 2);
  assert.equal(s.ball.pos.y, c.table.height / 2);
  assert.equal(s.phase, 'playing');
});

test('T18 launch angle is bounded away from vertical; vel.x is never 0', () => {
  // Sample the whole rng range, including the extremes that pick max angle.
  for (let i = 0; i <= 20; i++) {
    const t = i / 20;
    for (const side of [0.1, 0.9]) {
      const s = serveBall(makeState(c), c, seqRng([side, t]));
      assert.ok(Math.abs(s.ball.vel.x) > 1e-6, `vel.x non-zero (t=${t})`);
      // |vel.y/speed| = |sin(angle)| must stay below sin(maxBounceAngle) < 1.
      const speed = mag(s.ball.vel.x, s.ball.vel.y);
      assert.ok(Math.abs(s.ball.vel.y) / speed <= Math.sin(c.ball.maxBounceAngle) + 1e-9);
    }
  }
});

test('T19 serve is exactly reproducible for a fixed rng sequence', () => {
  const a = serveBall(makeState(c), c, seqRng([0.7, 0.33]));
  const b = serveBall(makeState(c), c, seqRng([0.7, 0.33]));
  assert.deepEqual(a.ball.vel, b.ball.vel);
});

test('T19b distinct rng sequences can produce distinct serves', () => {
  const left = serveBall(makeState(c), c, seqRng([0.1, 0.5])); // side -> left
  const right = serveBall(makeState(c), c, seqRng([0.9, 0.5])); // side -> right
  assert.ok(Math.sign(left.ball.vel.x) !== Math.sign(right.ball.vel.x));
});
