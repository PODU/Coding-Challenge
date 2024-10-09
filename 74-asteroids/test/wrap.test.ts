import { describe, expect, it } from "vitest";
import { wrap } from "../src/core/physics.js";
import { wrappedDist2 } from "../src/core/collisions.js";

const W = 100;
const H = 80;

describe("wrap (R4)", () => {
  it("wraps past each edge to the opposite side", () => {
    expect(wrap({ x: 105, y: 10 }, W, H)).toEqual({ x: 5, y: 10 }); // right -> left
    expect(wrap({ x: -5, y: 10 }, W, H)).toEqual({ x: 95, y: 10 }); // left -> right
    expect(wrap({ x: 10, y: 85 }, W, H)).toEqual({ x: 10, y: 5 }); // bottom -> top
    expect(wrap({ x: 10, y: -5 }, W, H)).toEqual({ x: 10, y: 75 }); // top -> bottom
  });

  it("wraps corners on both axes at once", () => {
    expect(wrap({ x: -1, y: -1 }, W, H)).toEqual({ x: 99, y: 79 });
  });

  it("leaves interior points untouched", () => {
    expect(wrap({ x: 50, y: 40 }, W, H)).toEqual({ x: 50, y: 40 });
  });
});

describe("wrappedDist2 (E1/E2)", () => {
  it("takes the seam distance, not the long way around", () => {
    // points at x=2 and x=98 are 4 apart across the seam, not 96.
    const d2 = wrappedDist2({ x: 2, y: 40 }, { x: 98, y: 40 }, W, H);
    expect(d2).toBe(16); // 4^2
  });

  it("matches direct distance when nearer the middle", () => {
    const d2 = wrappedDist2({ x: 40, y: 40 }, { x: 50, y: 40 }, W, H);
    expect(d2).toBe(100); // 10^2
  });
});
