/** A source of random numbers in [0, 1). */
export interface RNG {
  next(): number;
}

/** mulberry32, a small 32-bit PRNG. The same seed gives the same sequence. */
export function mulberry32(seed: number): RNG {
  let a = seed >>> 0;
  return {
    next(): number {
      a |= 0;
      a = (a + 0x6d2b79f5) | 0;
      let t = Math.imul(a ^ (a >>> 15), 1 | a);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    },
  };
}

/** A random seed from crypto.getRandomValues, or Math.random without it. */
export function randomSeed(): number {
  const g = globalThis as { crypto?: { getRandomValues?: (a: Uint32Array) => Uint32Array } };
  if (g.crypto && typeof g.crypto.getRandomValues === "function") {
    const buf = new Uint32Array(1);
    g.crypto.getRandomValues(buf);
    return buf[0] >>> 0;
  }
  return (Math.random() * 0xffffffff) >>> 0;
}

/** For tests: an RNG that returns `values` in order, repeating. */
export function scriptedRng(values: number[]): RNG {
  if (values.length === 0) throw new Error("scriptedRng requires at least one value");
  let i = 0;
  return {
    next(): number {
      const v = values[i % values.length];
      i += 1;
      return v;
    },
  };
}
