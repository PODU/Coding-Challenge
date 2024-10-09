import { describe, expect, it } from "vitest";
import { pointsFor } from "../src/core/scoring.js";
import { step } from "../src/core/step.js";
import { makeRng } from "../src/core/rng.js";
import { ASTEROID_RADIUS } from "../src/core/constants.js";
import { input, makeState, stillAsteroid, stillBullet } from "./helpers.js";
import type { AsteroidTier } from "../src/types.js";

const DT = 1 / 60;
const rng = () => makeRng(3);

describe("pointsFor (R9)", () => {
  it("smaller asteroids award more points", () => {
    expect(pointsFor("small")).toBeGreaterThan(pointsFor("medium"));
    expect(pointsFor("medium")).toBeGreaterThan(pointsFor("large"));
  });

  it("matches the tuning table", () => {
    expect(pointsFor("large")).toBe(20);
    expect(pointsFor("medium")).toBe(50);
    expect(pointsFor("small")).toBe(100);
  });
});

describe("scoring through step", () => {
  const cases: AsteroidTier[] = ["large", "medium", "small"];
  for (const tier of cases) {
    it(`awards pointsFor('${tier}') when a ${tier} is shot`, () => {
      const ast = stillAsteroid({
        tier,
        radius: ASTEROID_RADIUS[tier],
        pos: { x: 300, y: 300 },
      });
      const s = makeState({
        asteroids: [ast],
        bullets: [stillBullet({ x: 300, y: 300 })],
      });
      s.ship.invulnerable = 10;
      s.ship.pos = { x: 10, y: 10 };
      const { state } = step(s, input(), DT, rng());
      expect(state.score).toBe(pointsFor(tier));
    });
  }
});
