import { describe, it, expect } from "vitest";
import { mulberry32, scriptedRng } from "../src/core/rng.js";

describe("mulberry32", () => {
  it("is deterministic for a given seed", () => {
    const a = mulberry32(12345);
    const b = mulberry32(12345);
    const seqA = Array.from({ length: 8 }, () => a.next());
    const seqB = Array.from({ length: 8 }, () => b.next());
    expect(seqA).toEqual(seqB);
  });

  it("yields values in [0, 1)", () => {
    const r = mulberry32(7);
    for (let i = 0; i < 1000; i++) {
      const v = r.next();
      expect(v).toBeGreaterThanOrEqual(0);
      expect(v).toBeLessThan(1);
    }
  });

  it("produces different streams for different seeds", () => {
    const a = mulberry32(1).next();
    const b = mulberry32(2).next();
    expect(a).not.toEqual(b);
  });

  it("has a documented stable first value for seed 42", () => {
    // Pin the stream so regressions in the PRNG are caught.
    const v = mulberry32(42).next();
    expect(v).toBeCloseTo(0.6011037519201636, 12);
  });
});

describe("scriptedRng", () => {
  it("replays values then loops", () => {
    const r = scriptedRng([0.1, 0.9]);
    expect(r.next()).toBe(0.1);
    expect(r.next()).toBe(0.9);
    expect(r.next()).toBe(0.1);
  });
});
