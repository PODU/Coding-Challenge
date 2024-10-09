/**
 * All gameplay tuning in one place.
 *
 * Balance changes live here and never touch logic.
 */

import type { AsteroidTier } from "./entities.js";

/** Fixed simulation timestep (seconds). */
export const DT = 1 / 60;

/** Default play-field size (px). */
export const FIELD_WIDTH = 960;
export const FIELD_HEIGHT = 720;

export const SHIP_TURN_RATE = 4.0; // rad/s
export const SHIP_THRUST = 250; // px/s^2
export const SHIP_FRICTION = 0.7; // 1/s — vel *= e^(-k*dt)
export const SHIP_MAX_SPEED = 400; // px/s
export const SHIP_RADIUS = 12; // px (collision + nose offset)
export const RESPAWN_INVULN = 2.5; // s of post-spawn immunity
export const RESPAWN_DELAY = 1.5; // s the ship stays dead before respawn
export const START_LIVES = 3;

export const BULLET_SPEED = 550; // px/s, added to ship velocity
export const BULLET_TTL = 1.1; // s lifetime (bounds range)
export const BULLET_RADIUS = 2; // px
export const FIRE_COOLDOWN = 0.2; // s between shots
export const MAX_BULLETS = 4; // concurrent player bullets

export const ASTEROID_RADIUS: Record<AsteroidTier, number> = {
  large: 60,
  medium: 32,
  small: 16,
};
export const ASTEROID_SCORE: Record<AsteroidTier, number> = {
  large: 20,
  medium: 50,
  small: 100,
};
/** Tier reached after splitting; `small` has no child tier. */
export const ASTEROID_NEXT_TIER: Record<AsteroidTier, AsteroidTier | null> = {
  large: "medium",
  medium: "small",
  small: null,
};

export const ASTEROID_BASE_SPEED = 50; // px/s drift, wave 1
export const ASTEROID_WAVE_SPEED_INC = 8; // px/s added per wave
export const SPLIT_SPEED_MULT = 1.5; // children move 1.5x parent speed
export const SPLIT_CHILDREN = 2; // children per non-small hit
export const ASTEROID_SPIN_MAX = 1.5; // rad/s cosmetic spin (±)
export const ASTEROID_VERTS = 11; // shape vertices
export const ASTEROID_JITTER = 0.4; // ± fraction of radius per vertex

export const WAVE_BASE_COUNT = 4; // large asteroids on wave 1
export const SPAWN_SAFE_RADIUS = 150; // keep new asteroids clear of the ship
