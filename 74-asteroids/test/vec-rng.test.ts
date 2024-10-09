import { describe, expect, it } from "vitest";
import { add, fromAngle, limit, mag, rotate, scale } from "../src/core/vec2.js";
import { makeRng, range, unitVector } from "../src/core/rng.js";

describe("vec2 (7.1)", () => {
  it("add and scale", () => {
    expect(add({ x: 1, y: 2 }, { x: 3, y: -1 })).toEqual({ x: 4, y: 1 });
    expect(scale({ x: 2, y: -3 }, 2)).toEqual({ x: 4, y: -6 });
  });

  it("rotate by 90 degrees", () => {
    const r = rotate({ x: 1, y: 0 }, Math.PI / 2);
    expect(r.x).toBeCloseTo(0, 10);
    expect(r.y).toBeCloseTo(1, 10);
  });

  it("fromAngle is unit length", () => {
    expect(mag(fromAngle(0.9))).toBeCloseTo(1, 10);
  });

  it("limit clamps magnitude only when exceeded", () => {
    expect(mag(limit({ x: 30, y: 40 }, 10))).toBeCloseTo(10, 10);
    expect(limit({ x: 3, y: 4 }, 10)).toEqual({ x: 3, y: 4 });
  });
});

describe("rng (7.1)", () => {
  it("same seed -> identical sequence", () => {
    const a = makeRng(123);
    const b = makeRng(123);
    for (let i = 0; i < 50; i++) expect(a.next()).toBe(b.next());
  });

  it("different seeds -> different sequences", () => {
    const a = makeRng(1);
    const b = makeRng(2);
    let same = true;
    for (let i = 0; i < 20; i++) if (a.next() !== b.next()) same = false;
    expect(same).toBe(false);
  });

  it("next() stays in [0,1)", () => {
    const r = makeRng(7);
    for (let i = 0; i < 1000; i++) {
      const v = r.next();
      expect(v).toBeGreaterThanOrEqual(0);
      expect(v).toBeLessThan(1);
    }
  });

  it("range and unitVector respect bounds", () => {
    const r = makeRng(5);
    for (let i = 0; i < 100; i++) {
      const v = range(r, -3, 7);
      expect(v).toBeGreaterThanOrEqual(-3);
      expect(v).toBeLessThan(7);
    }
    expect(mag(unitVector(makeRng(8)))).toBeCloseTo(1, 10);
  });
});
