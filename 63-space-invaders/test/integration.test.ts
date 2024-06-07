import assert from 'node:assert/strict';
import { test } from 'node:test';
import { defaultConfig } from '../src/core/config.js';
import { aliveCount } from '../src/core/fleet.js';
import type { GameEvent } from '../src/core/types.js';
import { NO_INPUT, makeConfig, mutState, newGame, shootAlien } from './helpers.js';

function tinyConfig() {
  return makeConfig({
    fleet: { ...defaultConfig().fleet, cols: 3, rows: 2 },
    scoring: { rowPoints: [30, 10], ufoPoints: 0 },
    bunkers: { ...defaultConfig().bunkers, count: 0 },
    ufo: { ...defaultConfig().ufo, enabled: false },
  });
}

test('scripted mini-game: clears a wave (win path)', () => {
  const cfg = tinyConfig();
  const g = newGame(cfg);
  const s = mutState(g);
  const all: GameEvent[] = [];

  for (let r = 0; r < cfg.fleet.rows; r++) {
    for (let c = 0; c < cfg.fleet.cols; c++) {
      all.push(...shootAlien(g, r, c));
    }
  }

  assert.equal(all.filter((e) => e.type === 'alienKilled').length, 6);
  assert.equal(all.filter((e) => e.type === 'waveCleared').length, 1);
  assert.equal(s.wave, 2);
  assert.equal(aliveCount(s.fleet), 6, 'next wave repopulated');
  // 3 top-row (30) + 3 bottom-row (10)
  assert.equal(s.score, 3 * 30 + 3 * 10);
});

test('scripted mini-game: runs out of lives (loss path)', () => {
  const cfg = tinyConfig();
  const g = newGame(cfg);
  const s = mutState(g);
  const all: GameEvent[] = [];

  for (let i = 0; i < cfg.lives; i++) {
    s.bombs = [
      {
        pos: { x: s.ship.pos.x + s.ship.w / 2, y: s.ship.pos.y + 2 },
        vel: { x: 0, y: cfg.bomb.speed },
        w: 4,
        h: 12,
      },
    ];
    all.push(...g.step(NO_INPUT, 0));
  }

  assert.equal(all.filter((e) => e.type === 'playerHit').length, cfg.lives);
  assert.equal(all.filter((e) => e.type === 'gameOver').length, 1);
  assert.equal(s.phase, 'gameOver');
  assert.equal(s.ship.lives, 0);

  // reset restores a clean wave-1 state
  g.reset();
  assert.equal(mutState(g).phase, 'playing');
  assert.equal(mutState(g).wave, 1);
  assert.equal(mutState(g).score, 0);
});

test('full default game advances many ticks without throwing (smoke)', () => {
  const g = newGame();
  let fired = false;
  for (let i = 0; i < 2000; i++) {
    const input = {
      left: i % 200 < 100,
      right: i % 200 >= 100,
      fire: i % 30 === 0,
    };
    const ev = g.step(input, 1 / 120);
    if (ev.some((e) => e.type === 'shotFired')) fired = true;
    if (g.state.phase === 'gameOver') break;
  }
  assert.ok(fired, 'the player managed to fire at least once');
});
