import { describe, it, expect } from 'vitest';
import { createGame, reduce, COLS, ROWS } from '../src/core/index';
import type { Action, GameState } from '../src/core/index';

const SCRIPT: Action[] = [
  { type: 'START' },
  { type: 'MOVE', dir: -1 },
  { type: 'ROTATE', dir: 1 },
  { type: 'TICK', dtMs: 500 },
  { type: 'MOVE', dir: 1 },
  { type: 'SOFT_DROP' },
  { type: 'HARD_DROP' },
  { type: 'TICK', dtMs: 900 },
  { type: 'ROTATE', dir: -1 },
  { type: 'HARD_DROP' },
  { type: 'TICK', dtMs: 1200 },
  { type: 'HARD_DROP' },
];

function run(seed: number): GameState {
  let s = createGame({ seed });
  for (const a of SCRIPT) s = reduce(s, a);
  return s;
}

const VALID = new Set(['I', 'O', 'T', 'S', 'Z', 'J', 'L', 0]);

describe('determinism & invariants', () => {
  it('same seed + same actions => deep-equal state', () => {
    expect(run(2024)).toEqual(run(2024));
  });

  it('different seeds => generally different state', () => {
    expect(run(2024)).not.toEqual(run(99));
  });

  it('grid stays 20x10 with only valid cells after any action sequence', () => {
    let s = createGame({ seed: 7 });
    const actions: Action[] = [];
    // A long pseudo-deterministic action stream.
    for (let i = 0; i < 400; i++) {
      const pick = i % 6;
      if (pick === 0) actions.push({ type: 'TICK', dtMs: 120 });
      else if (pick === 1) actions.push({ type: 'MOVE', dir: i % 2 === 0 ? -1 : 1 });
      else if (pick === 2) actions.push({ type: 'ROTATE', dir: i % 3 === 0 ? 1 : -1 });
      else if (pick === 3) actions.push({ type: 'SOFT_DROP' });
      else if (pick === 4) actions.push({ type: 'HARD_DROP' });
      else actions.push({ type: 'TICK', dtMs: 60 });
    }

    s = reduce(s, { type: 'START' });
    for (const a of actions) {
      s = reduce(s, a);
      if (s.phase === 'gameover') s = reduce(s, { type: 'RESTART' });
      expect(s.grid).toHaveLength(ROWS);
      for (const row of s.grid) {
        expect(row).toHaveLength(COLS);
        for (const cell of row) expect(VALID.has(cell)).toBe(true);
      }
    }
  });
});
