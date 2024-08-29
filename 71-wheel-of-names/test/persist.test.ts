import { describe, it, expect } from "vitest";
import { encodeWheel, decodeWheel } from "../src/core/persist.js";
import { WheelDecodeError } from "../src/core/errors.js";

describe("persist", () => {
  it("round-trips a names-only wheel", () => {
    const s = { names: ["Alice", "Bob"] };
    expect(decodeWheel(encodeWheel(s))).toEqual(s);
  });

  it("round-trips weights and colours", () => {
    const s = { names: ["A", "B"], weights: [1, 2], colors: ["#aaa", "#bbb"] };
    expect(decodeWheel(encodeWheel(s))).toEqual(s);
  });

  it("omits optional fields so round-trip deep-equals", () => {
    const s = { names: ["x"] };
    const decoded = decodeWheel(encodeWheel(s));
    expect(decoded).toEqual(s);
    expect("weights" in decoded).toBe(false);
    expect("colors" in decoded).toBe(false);
  });

  it("throws on invalid JSON", () => {
    expect(() => decodeWheel("not json")).toThrow(WheelDecodeError);
  });

  it("throws on wrong shape", () => {
    expect(() => decodeWheel(JSON.stringify({ foo: 1 }))).toThrow(WheelDecodeError);
    expect(() => decodeWheel(JSON.stringify({ names: [1, 2] }))).toThrow(WheelDecodeError);
    expect(() => decodeWheel(JSON.stringify({ names: ["a"], weights: ["x"] }))).toThrow(
      WheelDecodeError,
    );
  });
});
