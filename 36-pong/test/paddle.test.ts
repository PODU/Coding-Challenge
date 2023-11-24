import { test } from 'node:test';
import assert from 'node:assert/strict';
import { movePlayerPaddle, clampPaddleY } from '../src/core/paddle.js';
import { cfg, makeState } from './helpers.js';

const c = cfg();

test('T11 input +1 moves down, -1 up, 0 unchanged (by speed*dt)', () => {
  const s = makeState(c, { leftPaddleY: 200 });
  assert.equal(movePlayerPaddle(s, 1, c, 0.1).leftPaddleY, 200 + c.paddle.speed * 0.1);
  assert.equal(movePlayerPaddle(s, -1, c, 0.1).leftPaddleY, 200 - c.paddle.speed * 0.1);
  assert.equal(movePlayerPaddle(s, 0, c, 0.1).leftPaddleY, 200);
});

test('T12 paddle clamps at the top (>= 0) and bottom (<= height - paddleH)', () => {
  const top = makeState(c, { leftPaddleY: 5 });
  assert.equal(movePlayerPaddle(top, -1, c, 1).leftPaddleY, 0, 'clamped at top');

  const maxY = c.table.height - c.paddle.height;
  const bottom = makeState(c, { leftPaddleY: maxY - 5 });
  assert.equal(movePlayerPaddle(bottom, 1, c, 1).leftPaddleY, maxY, 'clamped at bottom');

  assert.equal(clampPaddleY(-50, c), 0);
  assert.equal(clampPaddleY(99999, c), maxY);
});
