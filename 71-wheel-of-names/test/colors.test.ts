import { describe, it, expect } from "vitest";
import { assignColors } from "../src/core/colors.js";

describe("assignColors", () => {
  it("returns [] for n = 0", () => {
    expect(assignColors(0)).toEqual([]);
  });
  it("returns a single colour for n = 1", () => {
    expect(assignColors(1)).toHaveLength(1);
  });
  it("returns exactly n colours", () => {
    for (const n of [2, 3, 7, 12, 50]) {
      expect(assignColors(n)).toHaveLength(n);
    }
  });
  it("no colour equals its left neighbour and wrap differs (n >= 2)", () => {
    for (const n of [2, 3, 4, 7, 13, 50, 101]) {
      const cs = assignColors(n);
      for (let i = 1; i < n; i++) {
        expect(cs[i]).not.toEqual(cs[i - 1]);
      }
      expect(cs[n - 1]).not.toEqual(cs[0]); // wrap-around
    }
  });
});
