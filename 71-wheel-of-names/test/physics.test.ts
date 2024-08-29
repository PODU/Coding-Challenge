import { describe, it, expect } from "vitest";
import { buildSegments, normalize, TAU } from "../src/core/geometry.js";
import { planSpin, resolveWinner, easeOutCubic } from "../src/core/physics.js";
import { mulberry32 } from "../src/core/rng.js";
import { EmptyWheelError } from "../src/core/errors.js";

const names = (n: number): string[] => Array.from({ length: n }, (_, i) => `n${i}`);

describe("easeOutCubic", () => {
  it("hits endpoints and is monotonic + decelerating", () => {
    expect(easeOutCubic(0)).toBe(0);
    expect(easeOutCubic(1)).toBe(1);
    let prev = 0;
    let prevSlope = Infinity;
    for (let i = 1; i <= 100; i++) {
      const t = i / 100;
      const v = easeOutCubic(t);
      expect(v).toBeGreaterThanOrEqual(prev); // monotonic
      const slope = v - prev;
      expect(slope).toBeLessThanOrEqual(prevSlope + 1e-9); // decelerating
      prev = v;
      prevSlope = slope;
    }
  });
});

describe("planSpin / resolveWinner (R9 round-trip)", () => {
  it("throws EmptyWheelError on empty wheel", () => {
    expect(() => planSpin([], mulberry32(1), 0)).toThrow(EmptyWheelError);
    expect(() => resolveWinner([], 0)).toThrow(EmptyWheelError);
  });

  it("resolveWinner(targetAngle) === winnerIndex for many seeds and sizes", () => {
    for (const n of [1, 2, 3, 7, 50]) {
      const segs = buildSegments(names(n));
      for (let seed = 0; seed < 300; seed++) {
        const fromAngle = (seed % 13) * 0.37;
        const plan = planSpin(segs, mulberry32(seed), fromAngle);
        expect(resolveWinner(segs, plan.targetAngle)).toBe(plan.winnerIndex);
      }
    }
  });

  it("works for weighted segments too", () => {
    const segs = buildSegments(names(4), { weights: [3, 1, 4, 1] });
    for (let seed = 0; seed < 300; seed++) {
      const plan = planSpin(segs, mulberry32(seed * 7 + 1), 0.0);
      expect(resolveWinner(segs, plan.targetAngle)).toBe(plan.winnerIndex);
    }
  });

  it("spins forward by at least minTurns and has positive duration", () => {
    const segs = buildSegments(names(5));
    const fromAngle = 1.234;
    const plan = planSpin(segs, mulberry32(11), fromAngle, { minTurns: 4 });
    const turns = (plan.targetAngle - fromAngle) / TAU;
    expect(turns).toBeGreaterThanOrEqual(4);
    expect(plan.durationMs).toBeGreaterThan(0);
  });

  it("landing point stays strictly inside the winner's arc", () => {
    for (const n of [2, 3, 9]) {
      const segs = buildSegments(names(n));
      for (let seed = 0; seed < 200; seed++) {
        const plan = planSpin(segs, mulberry32(seed + 100), 0);
        const seg = segs[plan.winnerIndex];
        const local = normalize(-plan.targetAngle);
        expect(local).toBeGreaterThan(seg.startAngle - 1e-9);
        expect(local).toBeLessThan(seg.endAngle + 1e-9);
        // strictly interior (margin keeps it off the edges)
        expect(local).toBeGreaterThan(seg.startAngle);
        expect(local).toBeLessThan(seg.endAngle);
      }
    }
  });

  it("respects custom duration", () => {
    const segs = buildSegments(names(3));
    const plan = planSpin(segs, mulberry32(1), 0, { durationMs: 1234 });
    expect(plan.durationMs).toBe(1234);
  });
});
