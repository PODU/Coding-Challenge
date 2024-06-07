import assert from 'node:assert/strict';
import { test } from 'node:test';
import { defaultConfig } from '../src/core/config.js';
import { bunkerRect } from '../src/core/bunkers.js';
import type { GameEvent } from '../src/core/types.js';
import { NO_INPUT, makeConfig, mutState, newGame, shootAlien } from './helpers.js';

test('player shot destroys an alien, scores, emits alienKilled, removes shot (R9/R10)', () => {
  const g = newGame();
  const s = mutState(g);
  const ev = shootAlien(g, 0, 0);
  assert.equal(s.fleet.alive[0][0], false, 'alien dead');
  assert.equal(s.playerShot, null, 'shot consumed');
  const killed = ev.find((e) => e.type === 'alienKilled') as
    | Extract<GameEvent, { type: 'alienKilled' }>
    | undefined;
  assert.ok(killed);
  assert.equal(killed!.points, defaultConfig().scoring.rowPoints[0]);
  assert.equal(s.score, defaultConfig().scoring.rowPoints[0]);
});

test('row-tiered scoring: top row worth more than bottom (R10)', () => {
  const cfg = defaultConfig();
  const g1 = newGame(cfg);
  shootAlien(g1, 0, 3); // top
  const topScore = mutState(g1).score;

  const g2 = newGame(cfg);
  shootAlien(g2, cfg.fleet.rows - 1, 3); // bottom
  const botScore = mutState(g2).score;

  assert.ok(topScore > botScore, `${topScore} > ${botScore}`);
  assert.equal(topScore, cfg.scoring.rowPoints[0]);
  assert.equal(botScore, cfg.scoring.rowPoints[cfg.fleet.rows - 1]);
});

test('player shot erodes a bunker, emits bunkerHit, no score (R11)', () => {
  const g = newGame();
  const s = mutState(g);
  const b = s.bunkers[0];
  const br = bunkerRect(b);
  // shot centred on the bunker
  s.playerShot = {
    pos: { x: br.x + br.w / 2, y: br.y + br.h / 2 },
    vel: { x: 0, y: -1 },
    w: 2,
    h: 2,
  };
  const scoreBefore = s.score;
  const ev = g.step(NO_INPUT, 0);
  assert.equal(s.playerShot, null, 'shot consumed by bunker');
  assert.equal(ev.filter((e) => e.type === 'bunkerHit').length, 1);
  assert.equal(s.score, scoreBefore, 'no score for hitting a bunker');
  // exactly one cell eroded
  let intact = 0;
  for (let r = 0; r < b.rows; r++) for (let c = 0; c < b.cols; c++) if (b.cells[r][c]) intact++;
  assert.equal(intact, b.rows * b.cols - 1);
});

test('alien bomb hits the ship: life lost, playerHit emitted, bomb removed (R9/R2)', () => {
  const g = newGame();
  const s = mutState(g);
  const livesBefore = s.ship.lives;
  s.bombs = [
    {
      pos: { x: s.ship.pos.x + s.ship.w / 2, y: s.ship.pos.y + 2 },
      vel: { x: 0, y: defaultConfig().bomb.speed },
      w: 4,
      h: 12,
    },
  ];
  const ev = g.step(NO_INPUT, 0);
  assert.equal(s.ship.lives, livesBefore - 1);
  assert.equal(s.bombs.length, 0, 'bomb consumed');
  const hit = ev.find((e) => e.type === 'playerHit') as
    | Extract<GameEvent, { type: 'playerHit' }>
    | undefined;
  assert.ok(hit);
  assert.equal(hit!.livesLeft, livesBefore - 1);
});

test('alien bomb erodes a bunker top-down and is consumed', () => {
  const g = newGame();
  const s = mutState(g);
  const b = s.bunkers[1];
  const br = bunkerRect(b);
  const col = 3;
  s.bombs = [
    {
      pos: { x: br.x + col * b.cellSize.x + 1, y: br.y - 6 },
      vel: { x: 0, y: defaultConfig().bomb.speed },
      w: 2,
      h: 12,
    },
  ];
  const ev = g.step(NO_INPUT, 0);
  assert.equal(s.bombs.length, 0, 'bomb consumed');
  assert.equal(ev.filter((e) => e.type === 'bunkerHit').length, 1);
  // top-down erosion removes the topmost cell of that column first
  assert.equal(b.cells[0][col], false, 'top cell eroded');
  assert.equal(b.cells[b.rows - 1][col], true, 'bottom cell intact');
});

test('shot consumed by an alien cannot also hit a bunker (ordering)', () => {
  // Put a bunker directly behind an alien; shot overlaps both. Alien wins.
  const cfg = makeConfig({ bunkers: { ...defaultConfig().bunkers, y: 70 } });
  const g = newGame(cfg);
  const ev = shootAlien(g, 4, 0);
  assert.ok(ev.some((e) => e.type === 'alienKilled'));
  assert.ok(!ev.some((e) => e.type === 'bunkerHit'), 'bunker not also hit');
});
