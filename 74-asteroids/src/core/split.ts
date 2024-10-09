/**
 * Asteroid tier-down + child spawning.
 */

import {
  ASTEROID_BASE_SPEED,
  ASTEROID_NEXT_TIER,
  SPLIT_CHILDREN,
  SPLIT_SPEED_MULT,
} from "./constants.js";
import { makeAsteroid, type Asteroid } from "./entities.js";
import { range, type Rng } from "./rng.js";
import { fromAngle, mag, scale } from "./vec2.js";

/**
 * Split a hit asteroid into its children.
 *
 * - `small` → `[]` (fully destroyed).
 * - otherwise → `SPLIT_CHILDREN` next-tier rocks at the parent's center, moving
 *   `SPLIT_SPEED_MULT`× the parent speed, with headings spread apart so they
 *   separate immediately.
 */
export function splitAsteroid(a: Asteroid, rng: Rng): Asteroid[] {
  const next = ASTEROID_NEXT_TIER[a.tier];
  if (next === null) return [];

  // Faster than the parent; never slower than the wave base drift so a slow
  // parent still yields visibly moving children.
  const parentSpeed = Math.max(mag(a.vel), ASTEROID_BASE_SPEED);
  const childSpeed = parentSpeed * SPLIT_SPEED_MULT;

  const baseAngle = Math.atan2(a.vel.y, a.vel.x);
  const spread = range(rng, Math.PI / 6, Math.PI / 3);

  const children: Asteroid[] = [];
  for (let i = 0; i < SPLIT_CHILDREN; i++) {
    // Fan the children out symmetrically around the parent heading.
    const offset = spread * (i - (SPLIT_CHILDREN - 1) / 2);
    const jitter = range(rng, -Math.PI / 12, Math.PI / 12);
    const heading = baseAngle + offset + jitter;
    const vel = scale(fromAngle(heading), childSpeed);
    children.push(makeAsteroid(a.pos, vel, next, rng));
  }
  return children;
}
