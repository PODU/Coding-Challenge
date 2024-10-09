import { describe, expect, it } from "vitest";
import { splitAsteroid } from "../src/core/split.js";
import { makeAsteroid } from "../src/core/entities.js";
import { makeRng } from "../src/core/rng.js";
import { mag } from "../src/core/vec2.js";
import { ASTEROID_RADIUS, SPLIT_SPEED_MULT } from "../src/core/constants.js";

const rng = () => makeRng(99);

describe("splitAsteroid (R7, R8, E3, E4)", () => {
  it("large -> 2 medium", () => {
    const a = makeAsteroid({ x: 0, y: 0 }, { x: 40, y: 0 }, "large", rng());
    const kids = splitAsteroid(a, rng());
    expect(kids.length).toBe(2);
    expect(kids.every((k) => k.tier === "medium")).toBe(true);
    expect(kids.every((k) => k.radius === ASTEROID_RADIUS.medium)).toBe(true);
  });

  it("medium -> 2 small", () => {
    const a = makeAsteroid({ x: 0, y: 0 }, { x: 40, y: 0 }, "medium", rng());
    const kids = splitAsteroid(a, rng());
    expect(kids.length).toBe(2);
    expect(kids.every((k) => k.tier === "small")).toBe(true);
  });

  it("small -> [] (destroyed, no children)", () => {
    const a = makeAsteroid({ x: 0, y: 0 }, { x: 40, y: 0 }, "small", rng());
    expect(splitAsteroid(a, rng())).toEqual([]);
  });

  it("children are faster than the parent (~1.5x)", () => {
    const parentSpeed = 80;
    const a = makeAsteroid({ x: 0, y: 0 }, { x: parentSpeed, y: 0 }, "large", rng());
    const kids = splitAsteroid(a, rng());
    for (const k of kids) {
      expect(mag(k.vel)).toBeGreaterThan(parentSpeed);
      expect(mag(k.vel)).toBeCloseTo(parentSpeed * SPLIT_SPEED_MULT, 6);
    }
  });

  it("children separate in heading (E4)", () => {
    const a = makeAsteroid({ x: 0, y: 0 }, { x: 60, y: 0 }, "large", rng());
    const [k1, k2] = splitAsteroid(a, rng());
    const h1 = Math.atan2(k1!.vel.y, k1!.vel.x);
    const h2 = Math.atan2(k2!.vel.y, k2!.vel.x);
    expect(Math.abs(h1 - h2)).toBeGreaterThan(1e-3);
  });

  it("full chain large -> 2 medium -> 4 small (R8)", () => {
    const a = makeAsteroid({ x: 0, y: 0 }, { x: 40, y: 0 }, "large", rng());
    const r = rng();
    const mediums = splitAsteroid(a, r);
    const smalls = mediums.flatMap((m) => splitAsteroid(m, r));
    expect(mediums.length).toBe(2);
    expect(smalls.length).toBe(4);
    expect(smalls.every((s) => s.tier === "small")).toBe(true);
    expect(smalls.flatMap((s) => splitAsteroid(s, r))).toEqual([]);
  });
});
