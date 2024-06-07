import assert from 'node:assert/strict';
import { test } from 'node:test';
import { defaultConfig } from '../src/core/config.js';
import { bunkerRect, createBunkers, erodeBunker } from '../src/core/bunkers.js';
import type { Projectile } from '../src/core/types.js';

function shotAtColumn(bx: number, by: number, col: number, cellW: number): Projectile {
  return { pos: { x: bx + col * cellW + 1, y: by + 1 }, vel: { x: 0, y: -1 }, w: 2, h: 2 };
}

test('createBunkers makes the configured count, fully intact', () => {
  const cfg = defaultConfig();
  const bunkers = createBunkers(cfg);
  assert.equal(bunkers.length, cfg.bunkers.count);
  for (const b of bunkers) {
    for (let r = 0; r < b.rows; r++) for (let c = 0; c < b.cols; c++) assert.equal(b.cells[r][c], true);
  }
});

test('shots erode a column bottom-up; bombs erode top-down', () => {
  const cfg = defaultConfig();
  const col = 2;

  const b1 = createBunkers(cfg)[0];
  const r1 = bunkerRect(b1);
  // shot (fromBelow = true)
  const cell1 = erodeBunker(b1, shotAtColumn(r1.x, r1.y, col, b1.cellSize.x), true);
  assert.deepEqual(cell1, { x: col, y: b1.rows - 1 }, 'lowest cell eroded first');

  const b2 = createBunkers(cfg)[0];
  const r2 = bunkerRect(b2);
  // bomb (fromBelow = false)
  const cell2 = erodeBunker(b2, shotAtColumn(r2.x, r2.y, col, b2.cellSize.x), false);
  assert.deepEqual(cell2, { x: col, y: 0 }, 'topmost cell eroded first');
});

test('repeated shots erode the same column from the bottom up', () => {
  const cfg = defaultConfig();
  const b = createBunkers(cfg)[0];
  const r = bunkerRect(b);
  const col = 1;
  const ys: number[] = [];
  for (let i = 0; i < b.rows; i++) {
    const cell = erodeBunker(b, shotAtColumn(r.x, r.y, col, b.cellSize.x), true);
    assert.ok(cell);
    ys.push(cell!.y);
  }
  // rows eroded from bottom (rows-1) up to 0
  assert.deepEqual(ys, [b.rows - 1, b.rows - 2, b.rows - 3, b.rows - 4, b.rows - 5, b.rows - 6].slice(0, b.rows));
});

test('a fully eroded column lets the projectile pass through (no erosion)', () => {
  const cfg = defaultConfig();
  const b = createBunkers(cfg)[0];
  const r = bunkerRect(b);
  const col = 0;
  for (let i = 0; i < b.rows; i++) erodeBunker(b, shotAtColumn(r.x, r.y, col, b.cellSize.x), true);
  const again = erodeBunker(b, shotAtColumn(r.x, r.y, col, b.cellSize.x), true);
  assert.equal(again, null, 'no intact cell -> passes through gap');
});

test('a miss returns null', () => {
  const cfg = defaultConfig();
  const b = createBunkers(cfg)[0];
  const far: Projectile = { pos: { x: -999, y: -999 }, vel: { x: 0, y: -1 }, w: 2, h: 2 };
  assert.equal(erodeBunker(b, far, true), null);
});
