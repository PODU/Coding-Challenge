import { test } from 'node:test';
import assert from 'node:assert/strict';
import { computeAiPaddleMove } from '../src/core/ai.js';
import { stepGame } from '../src/core/game.js';
import { cfg, makeState, noRng, seqRng } from './helpers.js';

const c = cfg();

test('T13 ball clearly below the AI paddle -> move down (+1)', () => {
  const s = makeState(c, { rightPaddleY: 100, ball: { pos: { x: 400, y: 400 }, vel: { x: 1, y: 0 } } });
  assert.equal(computeAiPaddleMove(s, c, noRng), 1);
});

test('T14 ball clearly above the AI paddle -> move up (-1)', () => {
  const s = makeState(c, { rightPaddleY: 300, ball: { pos: { x: 400, y: 20 }, vel: { x: 1, y: 0 } } });
  assert.equal(computeAiPaddleMove(s, c, noRng), -1);
});

test('T15 ball within the dead-zone of the paddle centre -> hold (0)', () => {
  const center = 200 + c.paddle.height / 2;
  const s = makeState(c, { rightPaddleY: 200, ball: { pos: { x: 400, y: center + c.ai.deadZone - 1 }, vel: { x: 1, y: 0 } } });
  assert.equal(computeAiPaddleMove(s, c, noRng), 0);
});

test('T16 AI never moves faster than cfg.ai.speed over a step', () => {
  const dt = 1 / 120;
  const before = makeState(c, {
    rightPaddleY: 100,
    ball: { pos: { x: 400, y: 470 }, vel: { x: 50, y: 0 } }, // far below -> AI chases
  });
  const after = stepGame(before, { playerDir: 0, restart: false }, c, seqRng([0.5]), dt);
  const moved = Math.abs(after.rightPaddleY - before.rightPaddleY);
  assert.ok(moved <= c.ai.speed * dt + 1e-9, 'per-step move is speed-capped');
  assert.ok(moved > 0, 'but it did chase the ball');
});
