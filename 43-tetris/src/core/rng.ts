// mulberry32, written as a pure function of its state so a game can be
// replayed from its seed.

export interface RngResult {
  /** Pass this back in for the next draw. */
  state: number;
  /** In [0, 1). */
  value: number;
}

export function rngNext(state: number): RngResult {
  let a = (state + 0x6d2b79f5) | 0;
  let t = Math.imul(a ^ (a >>> 15), 1 | a);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  const value = ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  return { state: a | 0, value };
}
