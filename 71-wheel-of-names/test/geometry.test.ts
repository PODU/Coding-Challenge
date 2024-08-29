import { describe, it, expect } from "vitest";
import {
  buildSegments,
  angleToIndex,
  normalize,
  TAU,
} from "../src/core/geometry.js";

const names = (n: number): string[] => Array.from({ length: n }, (_, i) => `n${i}`);

describe("normalize", () => {
  it("maps angles into [0, TAU)", () => {
    expect(normalize(0)).toBe(0);
    expect(normalize(TAU)).toBeCloseTo(0, 12);
    expect(normalize(-0.1)).toBeCloseTo(TAU - 0.1, 12);
    expect(normalize(TAU * 3 + 1)).toBeCloseTo(1, 12);
  });
});

describe("buildSegments", () => {
  it("empty names -> []", () => {
    expect(buildSegments([])).toEqual([]);
  });
  it("equal arcs by default, tiling [0, TAU)", () => {
    const segs = buildSegments(names(4));
    expect(segs).toHaveLength(4);
    expect(segs[0].startAngle).toBe(0);
    expect(segs[3].endAngle).toBe(TAU);
    for (const s of segs) {
      expect(s.endAngle - s.startAngle).toBeCloseTo(TAU / 4, 12);
    }
    // contiguous
    for (let i = 1; i < segs.length; i++) {
      expect(segs[i].startAngle).toBeCloseTo(segs[i - 1].endAngle, 12);
    }
  });
  it("1 name -> single full-circle segment", () => {
    const segs = buildSegments(["solo"]);
    expect(segs).toHaveLength(1);
    expect(segs[0].startAngle).toBe(0);
    expect(segs[0].endAngle).toBe(TAU);
  });
  it("weights make arcs proportional", () => {
    const segs = buildSegments(names(3), { weights: [1, 2, 3] });
    const arcs = segs.map((s) => s.endAngle - s.startAngle);
    expect(arcs[0]).toBeCloseTo(TAU * (1 / 6), 12);
    expect(arcs[1]).toBeCloseTo(TAU * (2 / 6), 12);
    expect(arcs[2]).toBeCloseTo(TAU * (3 / 6), 12);
  });
  it("clamps non-positive / NaN weights to epsilon", () => {
    const segs = buildSegments(names(3), { weights: [0, -5, NaN] });
    // all clamped equal -> roughly equal thirds
    for (const s of segs) {
      expect(s.endAngle - s.startAngle).toBeCloseTo(TAU / 3, 6);
    }
  });
  it("uses provided colours when count matches", () => {
    const segs = buildSegments(names(2), { colors: ["#aaa", "#bbb"] });
    expect(segs.map((s) => s.color)).toEqual(["#aaa", "#bbb"]);
  });
});

describe("angleToIndex (R9)", () => {
  for (const n of [1, 2, 3, 7, 50]) {
    it(`maps start / mid / just-before-end correctly for n=${n}`, () => {
      const segs = buildSegments(names(n));
      for (const seg of segs) {
        const arc = seg.endAngle - seg.startAngle;
        expect(angleToIndex(segs, seg.startAngle)).toBe(seg.index);
        expect(angleToIndex(segs, seg.midAngle)).toBe(seg.index);
        expect(angleToIndex(segs, seg.endAngle - arc * 1e-6)).toBe(seg.index);
      }
    });
  }

  it("boundary angle maps to the STARTING segment (half-open)", () => {
    const segs = buildSegments(names(4));
    // boundary between seg0 and seg1 is at TAU/4
    expect(angleToIndex(segs, TAU / 4)).toBe(1);
    expect(angleToIndex(segs, TAU / 2)).toBe(2);
  });

  it("normalisation: +TAU*k and negatives map identically", () => {
    const segs = buildSegments(names(5));
    for (const seg of segs) {
      const base = seg.midAngle;
      expect(angleToIndex(segs, base + TAU * 3)).toBe(seg.index);
      expect(angleToIndex(segs, base - TAU * 2)).toBe(seg.index);
    }
  });
});
