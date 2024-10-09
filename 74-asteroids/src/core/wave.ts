/**
 * Spawn a wave of large asteroids and detect a cleared field.
 */

import {
  ASTEROID_BASE_SPEED,
  ASTEROID_WAVE_SPEED_INC,
  SPAWN_SAFE_RADIUS,
  WAVE_BASE_COUNT,
} from "./constants.js";
import { makeAsteroid, type Asteroid } from "./entities.js";
import { range, unitVector, type Rng } from "./rng.js";
import { scale, type Vec2 } from "./vec2.js";
import { wrappedDist2 } from "./collisions.js";
import type { GameState } from "../types.js";

/** Large asteroids in a given wave: 4 + (wave-1). */
export const waveCount = (wave: number): number => WAVE_BASE_COUNT + (wave - 1);

/** Drift speed for large asteroids in a given wave. */
export const waveSpeed = (wave: number): number =>
  ASTEROID_BASE_SPEED + (wave - 1) * ASTEROID_WAVE_SPEED_INC;

/**
 * Spawn a wave's large asteroids. Positions are rejection-sampled to stay
 * outside `SPAWN_SAFE_RADIUS` of the ship so nothing spawns on the player.
 */
export function spawnWave(
  wave: number,
  w: number,
  h: number,
  rng: Rng,
  shipPos: Vec2,
): Asteroid[] {
  const count = waveCount(wave);
  const speed = waveSpeed(wave);
  const safe2 = SPAWN_SAFE_RADIUS * SPAWN_SAFE_RADIUS;
  const asteroids: Asteroid[] = [];

  for (let i = 0; i < count; i++) {
    let pos: Vec2 = { x: 0, y: 0 };
    for (let tries = 0; tries < 100; tries++) {
      pos = { x: range(rng, 0, w), y: range(rng, 0, h) };
      if (wrappedDist2(pos, shipPos, w, h) >= safe2) break;
    }
    const vel = scale(unitVector(rng), speed);
    asteroids.push(makeAsteroid(pos, vel, "large", rng));
  }
  return asteroids;
}

export const isWaveCleared = (state: GameState): boolean =>
  state.asteroids.length === 0;
