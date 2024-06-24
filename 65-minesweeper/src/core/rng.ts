/** Passed into the engine so tests can use a seeded generator. */
export interface RNG {
  /** Returns an integer in [0, maxExclusive). */
  nextInt(maxExclusive: number): number;
}

/** Seeded mulberry32 generator, for tests that need an exact mine layout. */
export function mulberry32(seed: number): RNG {
  let a = seed >>> 0;
  return {
    nextInt(maxExclusive: number): number {
      if (maxExclusive <= 0) return 0;
      a = (a + 0x6d2b79f5) | 0;
      let t = Math.imul(a ^ (a >>> 15), 1 | a);
      t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
      const r = ((t ^ (t >>> 14)) >>> 0) / 4294967296;
      return Math.floor(r * maxExclusive);
    },
  };
}

/** Math.random, for real games. */
export function systemRng(): RNG {
  return {
    nextInt(maxExclusive: number): number {
      if (maxExclusive <= 0) return 0;
      return Math.floor(Math.random() * maxExclusive);
    },
  };
}
