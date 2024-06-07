import { erodeBunker } from './bunkers.js';
import type { GameConfig } from './config.js';
import { alienRect } from './fleet.js';
import {
  intersects,
  projectileRect,
  shipRect,
  ufoRect,
  type GameEvent,
  type GameState,
} from './types.js';

/**
 * Apply this tick's hits to `state`. Each projectile hits at most one thing,
 * checked in this order: the player's shot against aliens, then the UFO, then
 * bunkers; each bomb against the ship, then bunkers.
 */
export function resolveCollisions(
  state: GameState,
  config: GameConfig,
  events: GameEvent[],
): void {
  resolvePlayerShot(state, config, events);
  resolveBombs(state, config, events);
}

function resolvePlayerShot(
  state: GameState,
  config: GameConfig,
  events: GameEvent[],
): void {
  const shot = state.playerShot;
  if (!shot) return;
  const sRect = projectileRect(shot);
  const fleet = state.fleet;

  // If the shot overlaps several aliens, the lowest one is the one it reached
  // first.
  let bestRow = -1;
  let bestCol = -1;
  let bestY = -Infinity;
  for (let r = 0; r < fleet.rows; r++) {
    for (let c = 0; c < fleet.cols; c++) {
      if (!fleet.alive[r][c]) continue;
      const ar = alienRect(fleet, r, c);
      if (intersects(sRect, ar) && ar.y > bestY) {
        bestY = ar.y;
        bestRow = r;
        bestCol = c;
      }
    }
  }
  if (bestRow >= 0) {
    fleet.alive[bestRow][bestCol] = false;
    const points = config.scoring.rowPoints[bestRow] ?? 0;
    state.score += points;
    events.push({ type: 'alienKilled', row: bestRow, col: bestCol, points });
    state.playerShot = null;
    return;
  }

  if (state.ufo && intersects(sRect, ufoRect(state.ufo))) {
    state.score += state.ufo.points;
    events.push({ type: 'ufoKilled', points: state.ufo.points });
    state.ufo = null;
    state.playerShot = null;
    return;
  }

  for (let i = 0; i < state.bunkers.length; i++) {
    const cell = erodeBunker(state.bunkers[i], shot, true);
    if (cell) {
      events.push({ type: 'bunkerHit', bunker: i, cell });
      state.playerShot = null;
      return;
    }
  }
}

function resolveBombs(
  state: GameState,
  config: GameConfig,
  events: GameEvent[],
): void {
  const survivors: typeof state.bombs = [];
  const sRect = shipRect(state.ship);

  for (const bomb of state.bombs) {
    const bRect = projectileRect(bomb);

    if (
      config.bombAnnihilatesShot &&
      state.playerShot &&
      intersects(bRect, projectileRect(state.playerShot))
    ) {
      state.playerShot = null;
      continue;
    }

    if (intersects(bRect, sRect)) {
      state.ship.lives -= 1;
      events.push({ type: 'playerHit', livesLeft: Math.max(0, state.ship.lives) });
      continue;
    }

    let consumed = false;
    for (let i = 0; i < state.bunkers.length; i++) {
      const cell = erodeBunker(state.bunkers[i], bomb, false);
      if (cell) {
        events.push({ type: 'bunkerHit', bunker: i, cell });
        consumed = true;
        break;
      }
    }
    if (!consumed) survivors.push(bomb);
  }

  state.bombs = survivors;
}
