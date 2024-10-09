import { describe, expect, it } from "vitest";
import {
  applyShipControls,
  integrateBody,
  normalizeAngle,
} from "../src/core/physics.js";
import { makeShip } from "../src/core/entities.js";
import { mag } from "../src/core/vec2.js";
import {
  DT,
  SHIP_MAX_SPEED,
  SHIP_THRUST,
  SHIP_TURN_RATE,
} from "../src/core/constants.js";
import type { InputState } from "../src/types.js";

const NONE: InputState = {
  left: false,
  right: false,
  thrust: false,
  fire: false,
  start: false,
};

describe("integrateBody", () => {
  it("advances position by velocity * dt", () => {
    const p = integrateBody({ x: 0, y: 0 }, { x: 100, y: -50 }, 0.5);
    expect(p).toEqual({ x: 50, y: -25 });
  });
});

describe("ship rotation (R1)", () => {
  it("left rotates CCW, right rotates CW by TURN_RATE*dt", () => {
    const ship = makeShip({ x: 0, y: 0 });
    ship.angle = 1;
    const l = applyShipControls(ship, { ...NONE, left: true }, DT);
    const r = applyShipControls(ship, { ...NONE, right: true }, DT);
    expect(l.angle).toBeCloseTo(1 - SHIP_TURN_RATE * DT, 10);
    expect(r.angle).toBeCloseTo(1 + SHIP_TURN_RATE * DT, 10);
  });

  it("keeps angle normalized to [0, 2pi)", () => {
    const ship = makeShip({ x: 0, y: 0 });
    ship.angle = 0.01;
    const l = applyShipControls(ship, { ...NONE, left: true }, DT);
    expect(l.angle).toBeGreaterThanOrEqual(0);
    expect(l.angle).toBeLessThan(Math.PI * 2);
    expect(normalizeAngle(-0.5)).toBeCloseTo(Math.PI * 2 - 0.5, 10);
  });
});

describe("ship thrust + inertia (R2, R3)", () => {
  it("thrust along angle 0 builds +x velocity over N steps", () => {
    let ship = makeShip({ x: 0, y: 0 });
    ship.angle = 0;
    ship.invulnerable = 0;
    for (let i = 0; i < 5; i++) {
      ship = applyShipControls(ship, { ...NONE, thrust: true }, DT);
    }
    expect(ship.vel.x).toBeGreaterThan(0);
    expect(Math.abs(ship.vel.y)).toBeLessThan(1e-9);
    // roughly thrust*time minus a little friction
    expect(ship.vel.x).toBeLessThan(SHIP_THRUST * DT * 5);
    expect(ship.vel.x).toBeGreaterThan(SHIP_THRUST * DT * 5 * 0.9);
  });

  it("with no thrust, speed strictly decreases and never reverses", () => {
    let ship = makeShip({ x: 0, y: 0 });
    ship.angle = 0;
    ship.vel = { x: 200, y: 0 };
    let prev = mag(ship.vel);
    for (let i = 0; i < 20; i++) {
      ship = applyShipControls(ship, NONE, DT);
      const speed = mag(ship.vel);
      expect(speed).toBeLessThan(prev);
      expect(ship.vel.x).toBeGreaterThan(0); // no sign reversal
      prev = speed;
    }
  });

  it("clamps velocity to SHIP_MAX_SPEED", () => {
    const ship = makeShip({ x: 0, y: 0 });
    ship.angle = 0;
    // Start well above the cap; the clamp must pull it back to the ceiling.
    ship.vel = { x: SHIP_MAX_SPEED * 2, y: 0 };
    const next = applyShipControls(ship, NONE, DT);
    expect(mag(next.vel)).toBeLessThanOrEqual(SHIP_MAX_SPEED + 1e-6);
    expect(mag(next.vel)).toBeCloseTo(SHIP_MAX_SPEED, 6);

    // And thrusting never pushes it past the cap.
    let s = makeShip({ x: 0, y: 0 });
    s.angle = 0;
    for (let i = 0; i < 1000; i++) s = applyShipControls(s, { ...NONE, thrust: true }, DT);
    expect(mag(s.vel)).toBeLessThanOrEqual(SHIP_MAX_SPEED + 1e-6);
  });

  it("ticks down fire cooldown and invulnerability", () => {
    const ship = makeShip({ x: 0, y: 0 });
    ship.fireCooldown = 0.1;
    ship.invulnerable = 0.1;
    const next = applyShipControls(ship, NONE, DT);
    expect(next.fireCooldown).toBeCloseTo(0.1 - DT, 10);
    expect(next.invulnerable).toBeCloseTo(0.1 - DT, 10);
  });
});
