import type { Config, GameState } from '../src/core/types.js';
import { makeConfig } from '../src/core/config.js';

/** Deterministic RNG that yields the given values in order, then repeats the last. */
export function seqRng(values: number[]): () => number {
  let i = 0;
  return () => {
    const v = values[Math.min(i, values.length - 1)];
    i++;
    return v;
  };
}

/** An rng that always throws — proves a code path consumed no randomness. */
export const noRng = (): number => {
  throw new Error('rng should not have been called');
};

/** Test config (the production defaults, overridable per test). */
export function cfg(overrides: Partial<Config> = {}): Config {
  return makeConfig(overrides);
}

/** Build a GameState with sensible defaults, overridable per test. */
export function makeState(c: Config, overrides: Partial<GameState> = {}): GameState {
  const paddleTop = c.table.height / 2 - c.paddle.height / 2;
  return {
    ball: {
      pos: { x: c.table.width / 2, y: c.table.height / 2 },
      vel: { x: 0, y: 0 },
    },
    leftPaddleY: paddleTop,
    rightPaddleY: paddleTop,
    scoreLeft: 0,
    scoreRight: 0,
    phase: 'playing',
    winner: null,
    serveDelay: 0,
    ...overrides,
  };
}

export const mag = (x: number, y: number): number => Math.hypot(x, y);
