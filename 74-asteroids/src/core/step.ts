/**
 * The master reducer.
 *
 *   step(state, input, dt, rng) -> { state, events }
 *
 * Pure and deterministic for a fixed (state, input, dt, rng-stream). It never
 * mutates the caller's state object: it works on a deep clone and returns it.
 * Effects are emitted as events; the core never draws or plays sound.
 */

import {
  BULLET_RADIUS,
  BULLET_SPEED,
  BULLET_TTL,
  FIELD_HEIGHT,
  FIELD_WIDTH,
  FIRE_COOLDOWN,
  MAX_BULLETS,
  RESPAWN_DELAY,
  SHIP_RADIUS,
  START_LIVES,
} from "./constants.js";
import { circlesOverlap } from "./collisions.js";
import {
  makeBullet,
  makeShip,
  type Asteroid,
  type Bullet,
  type Ship,
} from "./entities.js";
import { applyShipControls, integrateBody, normalizeAngle, wrap } from "./physics.js";
import type { Rng } from "./rng.js";
import { pointsFor } from "./scoring.js";
import { splitAsteroid } from "./split.js";
import { spawnWave } from "./wave.js";
import { add, fromAngle, scale } from "./vec2.js";
import type { GameEvent, GameState, InputState } from "../types.js";

const center = (w: number, h: number) => ({ x: w / 2, y: h / 2 });

const cloneShip = (s: Ship): Ship => ({
  pos: { x: s.pos.x, y: s.pos.y },
  vel: { x: s.vel.x, y: s.vel.y },
  angle: s.angle,
  thrusting: s.thrusting,
  fireCooldown: s.fireCooldown,
  invulnerable: s.invulnerable,
  alive: s.alive,
});

const cloneBullet = (b: Bullet): Bullet => ({
  pos: { x: b.pos.x, y: b.pos.y },
  vel: { x: b.vel.x, y: b.vel.y },
  ttl: b.ttl,
});

const cloneAsteroid = (a: Asteroid): Asteroid => ({
  pos: { x: a.pos.x, y: a.pos.y },
  vel: { x: a.vel.x, y: a.vel.y },
  tier: a.tier,
  radius: a.radius,
  angle: a.angle,
  spin: a.spin,
  shape: a.shape.slice(),
});

export const cloneState = (s: GameState): GameState => ({
  phase: s.phase,
  ship: cloneShip(s.ship),
  bullets: s.bullets.map(cloneBullet),
  asteroids: s.asteroids.map(cloneAsteroid),
  score: s.score,
  lives: s.lives,
  wave: s.wave,
  width: s.width,
  height: s.height,
  respawnTimer: s.respawnTimer,
});

/** A fresh attract-mode state with a drifting demo field behind the title. */
export function createInitialState(
  rng: Rng,
  width = FIELD_WIDTH,
  height = FIELD_HEIGHT,
): GameState {
  const c = center(width, height);
  return {
    phase: "attract",
    ship: makeShip(c),
    bullets: [],
    asteroids: spawnWave(1, width, height, rng, c),
    score: 0,
    lives: START_LIVES,
    wave: 1,
    width,
    height,
    respawnTimer: 0,
  };
}

/** Reset to wave 1 and begin play (used by Start / Restart). */
function startNewGame(s: GameState, rng: Rng): void {
  const c = center(s.width, s.height);
  s.phase = "playing";
  s.ship = makeShip(c);
  s.bullets = [];
  s.score = 0;
  s.lives = START_LIVES;
  s.wave = 1;
  s.asteroids = spawnWave(1, s.width, s.height, rng, c);
  s.respawnTimer = 0;
}

export function step(
  state: GameState,
  input: InputState,
  dt: number,
  rng: Rng,
): { state: GameState; events: GameEvent[] } {
  const s = cloneState(state);
  const events: GameEvent[] = [];

  if (s.phase === "attract" || s.phase === "game-over") {
    if (input.start) {
      startNewGame(s, rng);
    } else {
      // Keep the background field drifting under the title.
      integrateAsteroids(s, dt);
      return { state: s, events };
    }
  }

  if (s.phase === "ship-destroyed") {
    s.respawnTimer = Math.max(0, s.respawnTimer - dt);
    if (s.respawnTimer <= 0) {
      s.ship = makeShip(center(s.width, s.height));
      s.phase = "playing";
    }
  }

  if (s.ship.alive) {
    s.ship = applyShipControls(s.ship, input, dt);
    s.ship.pos = wrap(integrateBody(s.ship.pos, s.ship.vel, dt), s.width, s.height);
    if (input.thrust) events.push({ type: "thrust" });

    // Firing (gated by cooldown + max concurrent bullets).
    if (input.fire && s.ship.fireCooldown <= 0 && s.bullets.length < MAX_BULLETS) {
      const dir = fromAngle(s.ship.angle);
      const nose = add(s.ship.pos, scale(dir, SHIP_RADIUS));
      const vel = add(s.ship.vel, scale(dir, BULLET_SPEED));
      s.bullets.push(makeBullet(nose, vel, BULLET_TTL));
      s.ship.fireCooldown = FIRE_COOLDOWN;
      events.push({ type: "bullet-fired" });
    }
  }

  for (const b of s.bullets) {
    b.pos = wrap(integrateBody(b.pos, b.vel, dt), s.width, s.height);
    b.ttl -= dt;
  }
  s.bullets = s.bullets.filter((b) => b.ttl > 0);

  integrateAsteroids(s, dt);

  // Mark hits first and remove them afterwards, so each bullet and rock is
  // consumed at most once.
  const consumedBullets = new Set<number>();
  const consumedAsteroids = new Set<number>();
  const children: Asteroid[] = [];
  for (let ai = 0; ai < s.asteroids.length; ai++) {
    if (consumedAsteroids.has(ai)) continue;
    const a = s.asteroids[ai] as Asteroid;
    for (let bi = 0; bi < s.bullets.length; bi++) {
      if (consumedBullets.has(bi)) continue;
      const b = s.bullets[bi] as Bullet;
      if (circlesOverlap(b.pos, BULLET_RADIUS, a.pos, a.radius, s.width, s.height)) {
        consumedBullets.add(bi);
        consumedAsteroids.add(ai);
        s.score += pointsFor(a.tier);
        events.push({ type: "asteroid-hit", tier: a.tier, points: pointsFor(a.tier) });
        for (const c of splitAsteroid(a, rng)) children.push(c);
        break; // this bullet is spent; move to the next asteroid
      }
    }
  }
  if (consumedAsteroids.size > 0) {
    s.asteroids = s.asteroids.filter((_, i) => !consumedAsteroids.has(i));
    s.bullets = s.bullets.filter((_, i) => !consumedBullets.has(i));
    for (const c of children) s.asteroids.push(c);
  }

  if (s.phase === "playing" && s.ship.alive && s.ship.invulnerable <= 0) {
    for (const a of s.asteroids) {
      if (circlesOverlap(s.ship.pos, SHIP_RADIUS, a.pos, a.radius, s.width, s.height)) {
        s.ship.alive = false;
        s.ship.thrusting = false;
        s.lives -= 1;
        events.push({ type: "ship-destroyed" });
        if (s.lives <= 0) {
          s.phase = "game-over";
          events.push({ type: "game-over" });
        } else {
          s.phase = "ship-destroyed";
          s.respawnTimer = RESPAWN_DELAY;
        }
        break;
      }
    }
  }

  // Only reached if the ship survived: game over and death take precedence.
  if (s.phase === "playing" && s.asteroids.length === 0) {
    const cleared = s.wave;
    s.wave += 1;
    events.push({ type: "wave-cleared", wave: cleared });
    s.asteroids = spawnWave(s.wave, s.width, s.height, rng, s.ship.pos);
  }

  return { state: s, events };
}

/** Move + wrap + spin every asteroid in place. */
function integrateAsteroids(s: GameState, dt: number): void {
  for (const a of s.asteroids) {
    a.pos = wrap(integrateBody(a.pos, a.vel, dt), s.width, s.height);
    a.angle = normalizeAngle(a.angle + a.spin * dt);
  }
}
