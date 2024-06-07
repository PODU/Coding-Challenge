import assert from 'node:assert/strict';
import { test } from 'node:test';
import { defaultConfig } from '../src/core/config.js';
import { aliveCount } from '../src/core/fleet.js';
import { NO_INPUT, mutState, newGame, shootAlien } from './helpers.js';

test('fleet reaching the player row ends the game even with lives left (R12)', () => {
  const g = newGame();
  const s = mutState(g);
  assert.equal(s.ship.lives, 3);
  s.fleet.origin.y = s.ship.pos.y; // shove the fleet onto the player row
  const ev = g.step(NO_INPUT, 0);
  assert.equal(s.phase, 'gameOver');
  assert.ok(ev.some((e) => e.type === 'gameOver'));
  assert.equal(s.ship.lives, 3, 'lose-by-descent is independent of lives');
});

test('losing the last life ends the game; terminal state is idempotent (R2)', () => {
  const g = newGame();
  const s = mutState(g);
  s.ship.lives = 1;
  s.bombs = [
    {
      pos: { x: s.ship.pos.x + s.ship.w / 2, y: s.ship.pos.y + 2 },
      vel: { x: 0, y: defaultConfig().bomb.speed },
      w: 4,
      h: 12,
    },
  ];
  const ev = g.step(NO_INPUT, 0);
  assert.equal(s.ship.lives, 0);
  assert.equal(s.phase, 'gameOver');
  assert.ok(ev.some((e) => e.type === 'gameOver'));

  // subsequent steps are no-ops
  const scoreBefore = s.score;
  const ev2 = g.step({ left: true, right: true, fire: true }, 0.5);
  assert.deepEqual(ev2, []);
  assert.equal(s.phase, 'gameOver');
  assert.equal(s.score, scoreBefore);
});

test('clearing the fleet wins the wave and starts a harder wave (R13)', () => {
  const g = newGame();
  const s = mutState(g);
  // leave a single alien alive
  for (let r = 0; r < s.fleet.rows; r++)
    for (let c = 0; c < s.fleet.cols; c++) s.fleet.alive[r][c] = false;
  s.fleet.alive[0][0] = true;
  const startY = s.fleet.origin.y;

  const ev = shootAlien(g, 0, 0);
  const cleared = ev.find((e) => e.type === 'waveCleared') as
    | { type: 'waveCleared'; wave: number }
    | undefined;
  assert.ok(cleared, 'waveCleared emitted');
  assert.equal(cleared!.wave, 2);
  assert.equal(s.wave, 2);
  assert.equal(s.phase, 'playing');
  assert.equal(aliveCount(s.fleet), 55, 'fleet repopulated');
  assert.equal(s.score, defaultConfig().scoring.rowPoints[0], 'score preserved across wave');
  assert.ok(s.fleet.origin.y > startY, 'next wave starts lower (harder)');
});

test('reset returns a clean wave-1 state', () => {
  const g = newGame();
  const s0 = mutState(g);
  s0.score = 999;
  s0.wave = 5;
  s0.ship.lives = 1;
  g.reset();
  const s = mutState(g);
  assert.equal(s.score, 0);
  assert.equal(s.wave, 1);
  assert.equal(s.ship.lives, defaultConfig().lives);
  assert.equal(aliveCount(s.fleet), 55);
  assert.equal(s.phase, 'playing');
});
