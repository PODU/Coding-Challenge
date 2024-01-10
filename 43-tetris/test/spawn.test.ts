import { describe, it, expect } from 'vitest';
import { createGame, reduce, emptyGrid } from '../src/core/index';
import type { GameState, PieceId } from '../src/core/index';
import { startGame } from './helpers';

/** Collect the spawn order of N pieces, resetting the grid so nothing stacks. */
function spawnOrder(seed: number, n: number): PieceId[] {
  let s: GameState = startGame(seed);
  const order: PieceId[] = [];
  for (let i = 0; i < n; i++) {
    order.push(s.active!.id);
    s = reduce(s, { type: 'HARD_DROP' });
    s = { ...s, grid: emptyGrid() }; // keep the well clear so no game over
  }
  return order;
}

describe('spawn & 7-bag randomizer', () => {
  it('emits all 7 pieces exactly once per bag', () => {
    const order = spawnOrder(123, 14);
    const first = order.slice(0, 7);
    const second = order.slice(7, 14);
    expect(new Set(first).size).toBe(7);
    expect(new Set(second).size).toBe(7);
  });

  it('nextQueue[0] matches the next spawned piece', () => {
    let s = startGame(7);
    const preview = s.nextQueue[0];
    s = reduce(s, { type: 'HARD_DROP' });
    expect(s.active!.id).toBe(preview);
  });

  it('is reproducible for a given seed', () => {
    expect(spawnOrder(999, 14)).toEqual(spawnOrder(999, 14));
  });

  it('different seeds generally differ', () => {
    const a = spawnOrder(1, 14).join('');
    const b = spawnOrder(2, 14).join('');
    expect(a).not.toBe(b);
  });

  it('spawns the active piece at the top in rotation 0', () => {
    const s = startGame(42);
    expect(s.active).not.toBeNull();
    expect(s.active!.rotation).toBe(0);
    expect(s.active!.row).toBe(0);
  });

  it('createGame starts on the welcome screen with no active piece', () => {
    const s = createGame({ seed: 1 });
    expect(s.phase).toBe('welcome');
    expect(s.active).toBeNull();
    expect(s.nextQueue.length).toBeGreaterThanOrEqual(1);
  });
});
