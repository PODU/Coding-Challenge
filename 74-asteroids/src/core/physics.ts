/**
 * Integration, toroidal wrap, and ship controls.
 *
 * Pure: each function returns new data and never mutates its arguments.
 */

import {
  SHIP_FRICTION,
  SHIP_MAX_SPEED,
  SHIP_THRUST,
  SHIP_TURN_RATE,
} from "./constants.js";
import type { Ship } from "./entities.js";
import { add, fromAngle, limit, scale, type Vec2 } from "./vec2.js";
import type { InputState } from "../types.js";

/** Euler integrate a position by a velocity over `dt`. */
export const integrateBody = (p: Vec2, v: Vec2, dt: number): Vec2 =>
  add(p, scale(v, dt));

/** Normalize an angle into [0, 2π). */
export const normalizeAngle = (a: number): number => {
  const twoPi = Math.PI * 2;
  let r = a % twoPi;
  if (r < 0) r += twoPi;
  return r;
};

/** Wrap a position onto the toroidal play-field [0,w) × [0,h). */
export const wrap = (p: Vec2, w: number, h: number): Vec2 => ({
  x: ((p.x % w) + w) % w,
  y: ((p.y % h) + h) % h,
});

/**
 * Apply rotation, thrust, inertial friction and speed clamp to a ship, and
 * tick down its per-frame timers. Returns a new ship; does not integrate
 * position (the master step does that after firing).
 */
export function applyShipControls(
  ship: Ship,
  input: InputState,
  dt: number,
): Ship {
  let angle = ship.angle;
  if (input.left) angle -= SHIP_TURN_RATE * dt; // CCW
  if (input.right) angle += SHIP_TURN_RATE * dt; // CW
  angle = normalizeAngle(angle);

  let vel: Vec2 = { x: ship.vel.x, y: ship.vel.y };
  if (input.thrust) {
    vel = add(vel, scale(fromAngle(angle), SHIP_THRUST * dt));
  }
  // Inertia: exponential decay keeps motion but bleeds speed.
  vel = scale(vel, Math.exp(-SHIP_FRICTION * dt));
  vel = limit(vel, SHIP_MAX_SPEED);

  return {
    ...ship,
    angle,
    vel,
    thrusting: input.thrust,
    fireCooldown: Math.max(0, ship.fireCooldown - dt),
    invulnerable: Math.max(0, ship.invulnerable - dt),
  };
}
