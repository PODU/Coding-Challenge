import assert from 'node:assert/strict';
import { test } from 'node:test';
import { defaultConfig } from '../src/core/config.js';
import { FIRE, NO_INPUT, makeConfig, mutState, newGame } from './helpers.js';

test('ship moves right and left under input (R1)', () => {
  const cfg = defaultConfig();
  const g = newGame(cfg);
  const x0 = mutState(g).ship.pos.x;

  g.step({ left: false, right: true, fire: false }, 0.1);
  const xRight = mutState(g).ship.pos.x;
  assert.ok(xRight > x0, 'moves right');
  assert.ok(Math.abs(xRight - x0 - cfg.ship.speed * 0.1) < 1e-6);

  g.step({ left: true, right: false, fire: false }, 0.1);
  assert.ok(mutState(g).ship.pos.x < xRight, 'moves left');
});

test('ship clamps at both edges (no wrap)', () => {
  const cfg = defaultConfig();
  const g = newGame(cfg);
  const s = mutState(g);

  for (let i = 0; i < 100; i++) g.step({ left: true, right: false, fire: false }, 0.1);
  assert.equal(s.ship.pos.x, cfg.bounds.x);

  for (let i = 0; i < 100; i++) g.step({ left: false, right: true, fire: false }, 0.1);
  assert.equal(s.ship.pos.x, cfg.bounds.x + cfg.bounds.w - s.ship.w);
});

test('fire cooldown gates re-firing (R8)', () => {
  const cfg = makeConfig({ ship: { ...defaultConfig().ship, fireCooldown: 0.5 } });
  const g = newGame(cfg);
  const s = mutState(g);

  const e1 = g.step(FIRE, 1 / 120);
  assert.equal(e1.filter((e) => e.type === 'shotFired').length, 1);
  assert.ok(s.ship.fireCooldown > 0);

  // shot leaves the field, then try to fire again before cooldown elapses
  s.playerShot = null;
  const e2 = g.step(FIRE, 0.1);
  assert.equal(e2.filter((e) => e.type === 'shotFired').length, 0, 'cooldown blocks');

  // wait out the cooldown
  for (let i = 0; i < 10; i++) {
    s.playerShot = null;
    g.step(NO_INPUT, 0.1);
  }
  s.playerShot = null;
  const e3 = g.step(FIRE, 0.01);
  assert.equal(e3.filter((e) => e.type === 'shotFired').length, 1, 'can fire after cooldown');
});
