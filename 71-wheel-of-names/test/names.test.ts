import { describe, it, expect } from "vitest";
import {
  parseNames,
  shuffle,
  sortNames,
  addName,
  removeAt,
  dedupe,
} from "../src/core/names.js";
import { mulberry32 } from "../src/core/rng.js";
import { IndexRangeError } from "../src/core/errors.js";

describe("parseNames", () => {
  it("trims and drops blank lines, preserves order", () => {
    expect(parseNames("  Alice \n\n Bob\n   \nCarol")).toEqual(["Alice", "Bob", "Carol"]);
  });
  it("handles CRLF, CR, and LF", () => {
    expect(parseNames("a\r\nb\rc\nd")).toEqual(["a", "b", "c", "d"]);
  });
  it("preserves internal whitespace", () => {
    expect(parseNames("Mary  Jane")).toEqual(["Mary  Jane"]);
  });
  it("empty input -> empty list", () => {
    expect(parseNames("")).toEqual([]);
    expect(parseNames("   \n  \n")).toEqual([]);
  });
});

describe("shuffle", () => {
  it("is a permutation (same multiset) and pure", () => {
    const input = ["a", "b", "c", "d", "e"];
    const out = shuffle(input, mulberry32(1));
    expect(input).toEqual(["a", "b", "c", "d", "e"]); // unmutated
    expect([...out].sort()).toEqual([...input].sort());
  });
  it("is deterministic for a fixed seed", () => {
    const input = ["a", "b", "c", "d", "e"];
    expect(shuffle(input, mulberry32(99))).toEqual(shuffle(input, mulberry32(99)));
  });
  it("has a low identity rate across many seeds", () => {
    const input = Array.from({ length: 8 }, (_, i) => String(i));
    let identity = 0;
    for (let s = 0; s < 200; s++) {
      const out = shuffle(input, mulberry32(s));
      if (out.every((v, i) => v === input[i])) identity++;
    }
    expect(identity).toBeLessThan(5);
  });
});

describe("sortNames", () => {
  it("is case-insensitive and locale-aware", () => {
    expect(sortNames(["banana", "Apple", "cherry", "apple"])).toEqual([
      "Apple",
      "apple",
      "banana",
      "cherry",
    ]);
  });
  it("is stable for equal keys", () => {
    // "apple" and "Apple" compare equal (base sensitivity) -> input order kept
    expect(sortNames(["apple", "Apple"])).toEqual(["apple", "Apple"]);
  });
  it("is numeric-aware", () => {
    expect(sortNames(["item10", "item2", "item1"])).toEqual(["item1", "item2", "item10"]);
  });
});

describe("addName / removeAt / dedupe", () => {
  it("addName appends purely", () => {
    const xs = ["a"];
    expect(addName(xs, "b")).toEqual(["a", "b"]);
    expect(xs).toEqual(["a"]);
  });
  it("removeAt removes index purely", () => {
    const xs = ["a", "b", "c"];
    expect(removeAt(xs, 1)).toEqual(["a", "c"]);
    expect(xs).toEqual(["a", "b", "c"]);
  });
  it("removeAt on the only element yields []", () => {
    expect(removeAt(["solo"], 0)).toEqual([]);
  });
  it("removeAt throws on out-of-range", () => {
    expect(() => removeAt(["a"], 1)).toThrow(IndexRangeError);
    expect(() => removeAt(["a"], -1)).toThrow(IndexRangeError);
  });
  it("dedupe keeps first occurrence, order-preserving", () => {
    expect(dedupe(["a", "b", "a", "c", "b"])).toEqual(["a", "b", "c"]);
  });
});
