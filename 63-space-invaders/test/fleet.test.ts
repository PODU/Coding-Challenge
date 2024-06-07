import assert from 'node:assert/strict';
import { test } from 'node:test';
import { defaultConfig } from '../src/core/config.js';
import {
  aliveColRange,
  alienRect,
  colStride,
  createFleet,
  marchSpeed,
  updateFleet,
} from '../src/core/fleet.js';
import type { GameEvent } from '../src/core/types.js';

test('fleet marches horizontally by speed*dt (R4)', () => {
  const cfg = defaultConfig();
  const f = createFleet(cfg, 1);
  const x0 = f.origin.x;
  const dt = 0.1;
  const speed = marchSpeed(f, cfg); // full fleet => baseSpeed
  updateFleet(f, dt, cfg, []);
  assert.ok(Math.abs(f.origin.x - (x0 + speed * dt)) < 1e-6);
  assert.equal(f.dir, 1);
});

function nearRightWall(cfg = defaultConfig()) {
  const f = createFleet(cfg, 1);
  const range = aliveColRange(f)!;
  const right = cfg.bounds.x + cfg.bounds.w;
  // place rightmost alive column 1px from the wall
  f.origin.x = right - (range.max * colStride(f) + f.cellSize.x) - 1;
  return { cfg, f };
}

test('right wall bounce flips dir, descends, speeds up, emits event (R5/R6)', () => {
  const { cfg, f } = nearRightWall();
  const y0 = f.origin.y;
  const speed0 = f.speed;
  const events: GameEvent[] = [];
  updateFleet(f, 0.5, cfg, events); // 0.5s * speed >> 1px -> bounce
  assert.equal(f.dir, -1, 'direction flipped');
  assert.ok(Math.abs(f.origin.y - (y0 + f.stepDown)) < 1e-6, 'descended by stepDown');
  assert.ok(f.speed > speed0, 'base speed increased');
  assert.equal(events.filter((e) => e.type === 'fleetDescended').length, 1);
  // clamped so rightmost alive column kisses the wall
  const range = aliveColRange(f)!;
  const rightEdge = f.origin.x + range.max * colStride(f) + f.cellSize.x;
  assert.ok(rightEdge <= cfg.bounds.x + cfg.bounds.w + 1e-6);
});

test('left wall bounce is symmetric', () => {
  const cfg = defaultConfig();
  const f = createFleet(cfg, 1);
  f.dir = -1;
  const range = aliveColRange(f)!;
  f.origin.x = cfg.bounds.x - range.min * colStride(f) + 1; // 1px from left wall
  const y0 = f.origin.y;
  const events: GameEvent[] = [];
  updateFleet(f, 0.5, cfg, events);
  assert.equal(f.dir, 1);
  assert.ok(Math.abs(f.origin.y - (y0 + f.stepDown)) < 1e-6);
  assert.equal(events.filter((e) => e.type === 'fleetDescended').length, 1);
});

test('edge detection uses outermost ALIVE column', () => {
  const cfg = defaultConfig();
  // Full fleet bounces:
  const full = nearRightWall(cfg).f;
  updateFleet(full, 0.5, cfg, []);
  assert.equal(full.dir, -1, 'full fleet bounced');

  // Kill the rightmost column -> same position should NOT bounce yet.
  const f = createFleet(cfg, 1);
  for (let r = 0; r < f.rows; r++) f.alive[r][f.cols - 1] = false;
  const right = cfg.bounds.x + cfg.bounds.w;
  // place where the FULL fleet's rightmost column would be at the wall
  f.origin.x = right - ((f.cols - 1) * colStride(f) + f.cellSize.x) - 1;
  const dirBefore = f.dir;
  updateFleet(f, 0.5, cfg, []);
  assert.equal(f.dir, dirBefore, 'trimmed fleet travels further before bouncing');
});

test('march speed strictly increases as aliens die, capped at max (R6)', () => {
  const cfg = defaultConfig();
  const f = createFleet(cfg, 1);
  const full = marchSpeed(f, cfg);
  f.alive[0][0] = false;
  const fewer = marchSpeed(f, cfg);
  assert.ok(fewer > full, 'attrition speeds the fleet up');

  // Cap check
  f.speed = 1e9;
  assert.equal(marchSpeed(f, cfg), cfg.fleet.maxSpeed);
});

test('fleet repopulates 55 alive at wave 1', () => {
  const cfg = defaultConfig();
  const f = createFleet(cfg, 1);
  let n = 0;
  for (let r = 0; r < f.rows; r++) for (let c = 0; c < f.cols; c++) if (f.alive[r][c]) n++;
  assert.equal(n, 55);
  // verify alienRect geometry monotonic
  assert.ok(alienRect(f, 0, 1).x > alienRect(f, 0, 0).x);
  assert.ok(alienRect(f, 1, 0).y > alienRect(f, 0, 0).y);
});
