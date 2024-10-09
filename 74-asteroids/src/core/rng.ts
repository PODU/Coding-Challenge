/**
 * Injected, seeded pseudo-random number generator (mulberry32).
 *
 * The core never touches `Math.random`; all nondeterminism flows through an
 * `Rng` instance so that an identical seed reproduces an identical stream.
 */

import { fromAngle, type Vec2 } from "./vec2.js";

export interface Rng {
  /** Returns a float in [0, 1). */
  next(): number;
}

/** mulberry32 — tiny, fast, well-distributed 32-bit PRNG. */
export function makeRng(seed: number): Rng {
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

/** Float in [min, max). */
export const range = (rng: Rng, min: number, max: number): number =>
  min + rng.next() * (max - min);

/** Pick a uniformly random element from a non-empty array. */
export const pick = <T>(rng: Rng, arr: readonly T[]): T => {
  const i = Math.min(arr.length - 1, Math.floor(rng.next() * arr.length));
  return arr[i] as T;
};

/** Random unit vector with uniformly distributed heading. */
export const unitVector = (rng: Rng): Vec2 => fromAngle(range(rng, 0, Math.PI * 2));
