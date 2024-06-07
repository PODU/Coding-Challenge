import type { Vec2 } from './types.js';

/** Random source for the core, so tests can control it. */
export interface RNG {
  /** Returns a float in [0, 1). */
  next(): number;
  state(): number;
  reseed(seed: number): void;
}

/** Mulberry32, a small seeded 32-bit PRNG. */
export class Mulberry32 implements RNG {
  private s: number;

  constructor(seed: number) {
    this.s = seed >>> 0;
  }

  next(): number {
    this.s = (this.s + 0x6d2b79f5) >>> 0;
    let t = this.s;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  }

  state(): number {
    return this.s >>> 0;
  }

  reseed(seed: number): void {
    this.s = seed >>> 0;
  }
}

/** Random integer in [0, n). */
export function randInt(rng: RNG, n: number): number {
  return Math.floor(rng.next() * n);
}

/** Random element of a non-empty array. */
export function pick<T>(rng: RNG, arr: readonly T[]): T {
  return arr[randInt(rng, arr.length)];
}

export type { Vec2 };
