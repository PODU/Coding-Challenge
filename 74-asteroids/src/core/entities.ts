/**
 * Entity types + pure factory functions.
 */

import {
  ASTEROID_JITTER,
  ASTEROID_RADIUS,
  ASTEROID_SPIN_MAX,
  ASTEROID_VERTS,
  RESPAWN_INVULN,
} from "./constants.js";
import { range, type Rng } from "./rng.js";
import type { Vec2 } from "./vec2.js";

export type AsteroidTier = "large" | "medium" | "small";

export type Ship = {
  pos: Vec2;
  vel: Vec2;
  angle: number; // radians, 0 = facing +x (right)
  thrusting: boolean; // for renderer flame
  fireCooldown: number; // s until next shot allowed
  invulnerable: number; // s of post-spawn immunity
  alive: boolean;
};

export type Bullet = {
  pos: Vec2;
  vel: Vec2;
  ttl: number; // s remaining before it expires
};

export type Asteroid = {
  pos: Vec2;
  vel: Vec2;
  tier: AsteroidTier;
  radius: number; // collision radius (by tier)
  angle: number; // visual rotation only
  spin: number; // rad/s visual spin
  shape: number[]; // per-vertex radius multipliers (cosmetic)
};

/** Build a ship at `pos`, facing up, with full post-spawn immunity. */
export function makeShip(pos: Vec2): Ship {
  return {
    pos: { x: pos.x, y: pos.y },
    vel: { x: 0, y: 0 },
    angle: -Math.PI / 2, // facing up
    thrusting: false,
    fireCooldown: 0,
    invulnerable: RESPAWN_INVULN,
    alive: true,
  };
}

export function makeBullet(pos: Vec2, vel: Vec2, ttl: number): Bullet {
  return { pos: { x: pos.x, y: pos.y }, vel: { x: vel.x, y: vel.y }, ttl };
}

/** Per-vertex radius multipliers (1 ± jitter) giving each rock a lumpy outline. */
export function makeAsteroidShape(rng: Rng): number[] {
  const shape: number[] = [];
  for (let i = 0; i < ASTEROID_VERTS; i++) {
    shape.push(1 - ASTEROID_JITTER + range(rng, 0, 2 * ASTEROID_JITTER));
  }
  return shape;
}

export function makeAsteroid(
  pos: Vec2,
  vel: Vec2,
  tier: AsteroidTier,
  rng: Rng,
): Asteroid {
  return {
    pos: { x: pos.x, y: pos.y },
    vel: { x: vel.x, y: vel.y },
    tier,
    radius: ASTEROID_RADIUS[tier],
    angle: range(rng, 0, Math.PI * 2),
    spin: range(rng, -ASTEROID_SPIN_MAX, ASTEROID_SPIN_MAX),
    shape: makeAsteroidShape(rng),
  };
}
