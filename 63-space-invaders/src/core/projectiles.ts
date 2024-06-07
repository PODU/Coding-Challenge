import type { GameConfig } from './config.js';
import {
  alienRect,
  aliveColumns,
  bottomMostAliveRow,
} from './fleet.js';
import type { RNG } from './rng.js';
import { randInt } from './rng.js';
import type { Fleet, GameEvent, Projectile, Ship } from './types.js';

/** Fire a shot upward from the middle of the ship. */
export function spawnPlayerShot(ship: Ship, config: GameConfig): Projectile {
  return {
    pos: {
      x: ship.pos.x + ship.w / 2 - config.shot.width / 2,
      y: ship.pos.y - config.shot.height,
    },
    vel: { x: 0, y: -config.shot.speed },
    w: config.shot.width,
    h: config.shot.height,
  };
}

/**
 * Every bomb interval, drop a bomb from the lowest alien in a random column.
 * The RNG is only used when a bomb actually drops.
 */
export function maybeDropBomb(
  fleet: Fleet,
  bombs: Projectile[],
  dt: number,
  config: GameConfig,
  rng: RNG,
  events: GameEvent[],
): void {
  fleet.bombTimer += dt;
  if (fleet.bombTimer < config.bomb.interval) return;
  fleet.bombTimer -= config.bomb.interval;

  if (bombs.length >= config.bomb.maxInFlight) return;

  const cols = aliveColumns(fleet);
  if (cols.length === 0) return;

  const col = cols[randInt(rng, cols.length)];
  const row = bottomMostAliveRow(fleet, col);
  if (row < 0) return;

  const r = alienRect(fleet, row, col);
  bombs.push({
    pos: { x: r.x + r.w / 2 - config.bomb.width / 2, y: r.y + r.h },
    vel: { x: 0, y: config.bomb.speed },
    w: config.bomb.width,
    h: config.bomb.height,
  });
  events.push({ type: 'bombDropped' });
}

/** Move the player's shot. Returns null once it is off the top of the field. */
export function advancePlayerShot(
  shot: Projectile | null,
  dt: number,
  config: GameConfig,
): Projectile | null {
  if (!shot) return null;
  shot.pos.x += shot.vel.x * dt;
  shot.pos.y += shot.vel.y * dt;
  if (shot.pos.y + shot.h < config.bounds.y) return null;
  return shot;
}

/** Move the bombs, dropping any that have left the bottom of the field. */
export function advanceBombs(
  bombs: Projectile[],
  dt: number,
  config: GameConfig,
): Projectile[] {
  const bottom = config.bounds.y + config.bounds.h;
  const next: Projectile[] = [];
  for (const b of bombs) {
    b.pos.x += b.vel.x * dt;
    b.pos.y += b.vel.y * dt;
    if (b.pos.y <= bottom) next.push(b);
  }
  return next;
}
