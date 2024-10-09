import { describe, expect, it } from "vitest";
import { circlesOverlap } from "../src/core/collisions.js";
import { step } from "../src/core/step.js";
import { makeRng } from "../src/core/rng.js";
import { pointsFor } from "../src/core/scoring.js";
import { ASTEROID_RADIUS } from "../src/core/constants.js";
import { input, makeState, stillAsteroid, stillBullet } from "./helpers.js";

const rng = () => makeRng(1234);

describe("circlesOverlap (R6, E1)", () => {
  it("detects direct overlap and wrap-seam overlap", () => {
    expect(circlesOverlap({ x: 0, y: 0 }, 5, { x: 6, y: 0 }, 5, 100, 100)).toBe(true);
    expect(circlesOverlap({ x: 0, y: 0 }, 5, { x: 30, y: 0 }, 5, 100, 100)).toBe(false);
    // ship at left edge, asteroid at right edge -> collide across seam
    expect(circlesOverlap({ x: 2, y: 50 }, 12, { x: 98, y: 50 }, 16, 100, 100)).toBe(true);
  });
});

describe("bullet ↔ asteroid via step (R6)", () => {
  it("consumes both bullet and asteroid and awards points", () => {
    const ast = stillAsteroid({ tier: "large", pos: { x: 300, y: 300 } });
    const bul = stillBullet({ x: 300, y: 300 });
    const s = makeState({ asteroids: [ast], bullets: [bul], ship: shipAway() });
    const { state, events } = step(s, input(), 1 / 60, rng());

    expect(state.bullets.length).toBe(0);
    expect(state.score).toBe(pointsFor("large"));
    expect(events.some((e) => e.type === "asteroid-hit")).toBe(true);
  });

  it("multi-hit safety: one bullet consumed once even with two overlapping rocks (E5)", () => {
    const a1 = stillAsteroid({ tier: "small", pos: { x: 300, y: 300 }, radius: ASTEROID_RADIUS.small });
    const a2 = stillAsteroid({ tier: "small", pos: { x: 305, y: 300 }, radius: ASTEROID_RADIUS.small });
    const bul = stillBullet({ x: 302, y: 300 });
    const s = makeState({ asteroids: [a1, a2], bullets: [bul], ship: shipAway() });
    const { state } = step(s, input(), 1 / 60, rng());

    // exactly one small destroyed (one bullet, one target), the other remains
    expect(state.asteroids.length).toBe(1);
    expect(state.score).toBe(pointsFor("small"));
  });
});

// keep the ship far from the test asteroids and intangible
function shipAway() {
  return {
    pos: { x: 10, y: 10 },
    vel: { x: 0, y: 0 },
    angle: 0,
    thrusting: false,
    fireCooldown: 0,
    invulnerable: 10,
    alive: true,
  };
}
