import { describe, it, expect } from "vitest";
import { buildSegments } from "../src/core/geometry.js";
import { pickWinner } from "../src/core/select.js";
import { mulberry32, scriptedRng } from "../src/core/rng.js";
import { EmptyWheelError } from "../src/core/errors.js";

const names = (n: number): string[] => Array.from({ length: n }, (_, i) => `n${i}`);

describe("pickWinner", () => {
  it("throws EmptyWheelError on empty wheel", () => {
    expect(() => pickWinner([], mulberry32(1))).toThrow(EmptyWheelError);
  });

  it("always returns 0 for a single segment", () => {
    const segs = buildSegments(["solo"]);
    for (const v of [0, 0.5, 0.999]) {
      expect(pickWinner(segs, scriptedRng([v]))).toBe(0);
    }
  });

  it("is fair across N equal segments (within tolerance)", () => {
    const N = 6;
    const segs = buildSegments(names(N));
    const counts = new Array(N).fill(0);
    const trials = 60000;
    const rng = mulberry32(2024);
    for (let i = 0; i < trials; i++) counts[pickWinner(segs, rng)]++;
    const expected = trials / N;
    for (const c of counts) {
      expect(Math.abs(c - expected) / expected).toBeLessThan(0.05);
    }
  });

  it("respects weights [1,2,3] -> ~1/6, 2/6, 3/6", () => {
    const segs = buildSegments(names(3), { weights: [1, 2, 3] });
    const counts = [0, 0, 0];
    const trials = 60000;
    const rng = mulberry32(7);
    for (let i = 0; i < trials; i++) counts[pickWinner(segs, rng)]++;
    expect(counts[0] / trials).toBeCloseTo(1 / 6, 1);
    expect(counts[1] / trials).toBeCloseTo(2 / 6, 1);
    expect(counts[2] / trials).toBeCloseTo(3 / 6, 1);
  });

  it("is deterministic for a fixed seed", () => {
    const segs = buildSegments(names(10));
    const a = Array.from({ length: 20 }, () => 0);
    const rngA = mulberry32(555);
    const seqA = a.map(() => pickWinner(segs, rngA));
    const rngB = mulberry32(555);
    const seqB = a.map(() => pickWinner(segs, rngB));
    expect(seqA).toEqual(seqB);
  });

  it("treats duplicate labels as independent indices", () => {
    const segs = buildSegments(["x", "x", "y"]);
    const counts = [0, 0, 0];
    const rng = mulberry32(3);
    for (let i = 0; i < 30000; i++) counts[pickWinner(segs, rng)]++;
    // each ~1/3
    for (const c of counts) expect(c / 30000).toBeCloseTo(1 / 3, 1);
  });
});
