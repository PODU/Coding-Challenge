import type { GameConfig } from './config.js';
import type { RNG } from './rng.js';
import type { GameEvent, Ufo } from './types.js';

export interface UfoController {
  timer: number;
}

export function createUfoController(): UfoController {
  return { timer: 0 };
}

/**
 * Every `interval` seconds with no UFO on screen, send one across the top from
 * a random side.
 */
export function maybeSpawnUfo(
  current: Ufo | null,
  ctrl: UfoController,
  dt: number,
  config: GameConfig,
  rng: RNG,
  events: GameEvent[],
): Ufo | null {
  if (!config.ufo.enabled) return current;
  if (current) return current;

  ctrl.timer += dt;
  if (ctrl.timer < config.ufo.interval) return current;
  ctrl.timer -= config.ufo.interval;

  const fromLeft = rng.next() < 0.5;
  const u = config.ufo;
  events.push({ type: 'ufoSpawned' });
  return {
    pos: { x: fromLeft ? config.bounds.x - u.width : config.bounds.x + config.bounds.w, y: u.y },
    w: u.width,
    h: u.height,
    vel: { x: fromLeft ? u.speed : -u.speed, y: 0 },
    points: config.scoring.ufoPoints,
  };
}

/** Move the UFO. Returns null once it has left the field. */
export function advanceUfo(
  ufo: Ufo | null,
  dt: number,
  config: GameConfig,
): Ufo | null {
  if (!ufo) return null;
  ufo.pos.x += ufo.vel.x * dt;
  const left = config.bounds.x - ufo.w;
  const right = config.bounds.x + config.bounds.w;
  if (ufo.pos.x > right || ufo.pos.x < left) return null;
  return ufo;
}
