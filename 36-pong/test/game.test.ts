import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createGame, stepGame } from '../src/core/game.js';
import { cfg, makeState, seqRng } from './helpers.js';
import type { InputState } from '../src/core/types.js';

const c = cfg();
const NEUTRAL: InputState = { playerDir: 0, restart: false };
const rng = () => seqRng([0.5, 0.5]); // fresh deterministic rng per call

test('T20 ball exiting the left edge gives the right player a point, then re-serves', () => {
  // Paddle parked at the top; ball aimed low so it misses and exits left.
  const s = makeState(c, {
    leftPaddleY: 0,
    rightPaddleY: 0,
    ball: { pos: { x: 5, y: 400 }, vel: { x: -400, y: 0 } },
  });
  const next = stepGame(s, NEUTRAL, c, rng(), 0.05);
  assert.equal(next.scoreRight, 1);
  assert.equal(next.scoreLeft, 0);
  assert.equal(next.phase, 'serving');
  assert.equal(next.ball.pos.x, c.table.width / 2, 're-served to centre');
  assert.equal(next.ball.pos.y, c.table.height / 2);
});

test('T21 ball exiting the right edge gives the left player a point', () => {
  const s = makeState(c, {
    leftPaddleY: 0,
    rightPaddleY: 0,
    ball: { pos: { x: c.table.width - 5, y: 400 }, vel: { x: 400, y: 0 } },
  });
  const next = stepGame(s, NEUTRAL, c, rng(), 0.05);
  assert.equal(next.scoreLeft, 1);
  assert.equal(next.scoreRight, 0);
  assert.equal(next.phase, 'serving');
});

test('T22 a single exit yields exactly one point (no double counting)', () => {
  const s = makeState(c, {
    leftPaddleY: 0,
    ball: { pos: { x: 5, y: 400 }, vel: { x: -400, y: 0 } },
  });
  const next = stepGame(s, NEUTRAL, c, rng(), 0.05);
  assert.equal(next.scoreRight + next.scoreLeft, 1);
});

test('T23 reaching the winning score ends the game with the right winner', () => {
  const s = makeState(c, {
    leftPaddleY: 0,
    scoreRight: c.winningScore - 1,
    ball: { pos: { x: 5, y: 400 }, vel: { x: -400, y: 0 } },
  });
  const next = stepGame(s, NEUTRAL, c, rng(), 0.05);
  assert.equal(next.scoreRight, c.winningScore);
  assert.equal(next.phase, 'gameOver');
  assert.equal(next.winner, 'right');
});

test('T24 in gameOver, further steps do not change the score', () => {
  const over = makeState(c, {
    phase: 'gameOver',
    winner: 'left',
    scoreLeft: c.winningScore,
    scoreRight: 3,
    ball: { pos: { x: 5, y: 400 }, vel: { x: -400, y: 0 } },
  });
  const next = stepGame(over, NEUTRAL, c, rng(), 0.05);
  assert.equal(next.scoreLeft, c.winningScore);
  assert.equal(next.scoreRight, 3);
  assert.equal(next.phase, 'gameOver');
});

test('T25 restart only acts in gameOver; it yields a fresh 0-0 serving game', () => {
  const over = makeState(c, { phase: 'gameOver', winner: 'right', scoreLeft: 4, scoreRight: 11 });
  const fresh = stepGame(over, { playerDir: 0, restart: true }, c, rng(), 0.05);
  assert.equal(fresh.scoreLeft, 0);
  assert.equal(fresh.scoreRight, 0);
  assert.equal(fresh.phase, 'serving');
  assert.equal(fresh.winner, null);

  // restart while playing is ignored: the game just steps normally.
  const playing = makeState(c, { ball: { pos: { x: 400, y: 240 }, vel: { x: 100, y: 0 } } });
  const stepped = stepGame(playing, { playerDir: 0, restart: true }, c, rng(), 0.05);
  assert.equal(stepped.phase, 'playing');
  assert.equal(stepped.scoreLeft, 0);
  assert.equal(stepped.scoreRight, 0);
});

test('T26 determinism: identical (state, input, rng-seq, dt) -> identical next state', () => {
  // Use a serving state whose delay elapses this step, so serveBall consumes rng.
  const base = makeState(c, { phase: 'serving', serveDelay: 0.01 });
  const a = stepGame(base, NEUTRAL, c, seqRng([0.3, 0.8]), 0.05);
  const b = stepGame(base, NEUTRAL, c, seqRng([0.3, 0.8]), 0.05);
  assert.deepEqual(a, b);
  assert.equal(a.phase, 'playing', 'serve fired this step');
});

test('createGame starts 0-0, centred, serving', () => {
  const g = createGame(c, rng());
  assert.equal(g.scoreLeft, 0);
  assert.equal(g.scoreRight, 0);
  assert.equal(g.phase, 'serving');
  assert.equal(g.winner, null);
  assert.equal(g.ball.pos.x, c.table.width / 2);
});
