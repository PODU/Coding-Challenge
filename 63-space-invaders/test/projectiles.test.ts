import assert from 'node:assert/strict';
import { test } from 'node:test';
import { defaultConfig } from '../src/core/config.js';
import { advanceBombs } from '../src/core/projectiles.js';
import type { Projectile } from '../src/core/types.js';
import { FIRE, NO_INPUT, makeConfig, mutState, newGame } from './helpers.js';

test('fire spawns a single upward shot and emits shotFired (R8)', () => {
  const g = newGame();
  const ev = g.step(FIRE, 1 / 120);
  assert.equal(ev.filter((e) => e.type === 'shotFired').length, 1);
  const shot = mutState(g).playerShot;
  assert.ok(shot, 'shot exists');
  assert.ok(shot!.vel.y < 0, 'travels upward');
});

test('second fire is ignored while a shot is in flight (R8)', () => {
  const g = newGame();
  g.step(FIRE, 1 / 120);
  const ev2 = g.step(FIRE, 1 / 120);
  assert.equal(ev2.filter((e) => e.type === 'shotFired').length, 0);
  // still exactly one shot
  assert.ok(mutState(g).playerShot);
});

test('bombs advance downward and are culled off the bottom', () => {
  const cfg = defaultConfig();
  const bottom = cfg.bounds.y + cfg.bounds.h;
  const bombs: Projectile[] = [
    { pos: { x: 100, y: 100 }, vel: { x: 0, y: cfg.bomb.speed }, w: 4, h: 12 },
    { pos: { x: 200, y: bottom - 1 }, vel: { x: 0, y: cfg.bomb.speed }, w: 4, h: 12 },
  ];
  const next = advanceBombs(bombs, 0.1, cfg);
  assert.ok(Math.abs(next[0].pos.y - (100 + cfg.bomb.speed * 0.1)) < 1e-6);
  assert.equal(next.length, 1, 'the off-bottom bomb was culled');
});

test('bomb drops are deterministic for a fixed seed (R7)', () => {
  const cfg = makeConfig({
    bomb: { ...defaultConfig().bomb, interval: 0.3, maxInFlight: 50 },
    ufo: { ...defaultConfig().ufo, enabled: false },
  });

  function run(seed: number): number[] {
    const g = newGame(cfg, seed);
    const xs: number[] = [];
    for (let i = 0; i < 60; i++) {
      const ev = g.step(NO_INPUT, 0.1);
      if (ev.some((e) => e.type === 'bombDropped')) {
        const bombs = mutState(g).bombs;
        xs.push(bombs[bombs.length - 1].pos.x);
      }
    }
    return xs;
  }

  const a = run(12345);
  const b = run(12345);
  const c = run(67890);

  assert.ok(a.length >= 5, 'several bombs dropped');
  assert.deepEqual(a, b, 'same seed -> identical drop columns');
  assert.notDeepEqual(a, c, 'different seed -> different sequence');
});
