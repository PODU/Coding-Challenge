// The game reducer. It never reads the clock or Math.random: time comes in as
// TICK deltas and randomness from the seeded RNG, so games are reproducible.

import type { Action, ActivePiece, GameState, PieceId } from './types';
import { collides, lock, fullRows, clearRows, emptyGrid } from './board';
import { tryRotate } from './rotation';
import {
  lineScore,
  nextLevel,
  gravityForLevel,
  SOFT_DROP_POINTS,
  HARD_DROP_POINTS,
} from './scoring';
import { SPAWN, PIECES } from './tetrominoes';
import { rngNext } from './rng';

const DEFAULT_SEED = 0x1a2b3c4d;
/** How many upcoming pieces the preview shows. */
const PREVIEW = 1;

interface BagDraw {
  bag: PieceId[];
  state: number;
}

/** A shuffled 7-bag (Fisher-Yates). */
function makeBag(rngState: number): BagDraw {
  const bag = PIECES.slice();
  let state = rngState;
  for (let i = bag.length - 1; i > 0; i--) {
    const r = rngNext(state);
    state = r.state;
    const j = Math.floor(r.value * (i + 1));
    [bag[i], bag[j]] = [bag[j], bag[i]];
  }
  return { bag, state };
}

interface QueueFill {
  bag: PieceId[];
  rngState: number;
  queue: PieceId[];
}

/** Top the queue up to `minLen` pieces, opening a new bag when one runs out. */
function ensureQueue(
  bag: PieceId[],
  rngState: number,
  queue: PieceId[],
  minLen: number,
): QueueFill {
  let b = bag.slice();
  let q = queue.slice();
  let s = rngState;
  while (q.length < minLen) {
    if (b.length === 0) {
      const made = makeBag(s);
      b = made.bag;
      s = made.state;
    }
    q.push(b[0]);
    b = b.slice(1);
  }
  return { bag: b, rngState: s, queue: q };
}

function spawnPiece(id: PieceId): ActivePiece {
  return { id, rotation: 0, row: SPAWN[id].row, col: SPAWN[id].col };
}

export function createGame(opts?: { seed?: number; simpleRotation?: boolean }): GameState {
  const seed = (opts?.seed ?? DEFAULT_SEED) | 0;
  const init = ensureQueue([], seed, [], PREVIEW);
  return {
    grid: emptyGrid(),
    active: null,
    nextQueue: init.queue,
    bag: init.bag,
    rngState: init.rngState,
    score: 0,
    lines: 0,
    level: 0,
    phase: 'welcome',
    gravityMs: gravityForLevel(0),
    dropAccumulatorMs: 0,
    simpleRotation: opts?.simpleRotation ?? false,
  };
}

/** Spawn the next piece. It is game over if the piece collides on spawn. */
function spawnNext(state: GameState): GameState {
  const filledIn = ensureQueue(state.bag, state.rngState, state.nextQueue, PREVIEW + 1);
  const id = filledIn.queue[0];
  const rest = filledIn.queue.slice(1);
  const refilled = ensureQueue(filledIn.bag, filledIn.rngState, rest, PREVIEW);
  const active = spawnPiece(id);
  const gameOver = collides(state.grid, active);
  return {
    ...state,
    active,
    nextQueue: refilled.queue,
    bag: refilled.bag,
    rngState: refilled.rngState,
    phase: gameOver ? 'gameover' : 'playing',
    dropAccumulatorMs: 0,
  };
}

/** Lock the piece, clear lines, update score and level, and spawn the next. */
function lockActive(state: GameState, piece: ActivePiece): GameState {
  const locked = lock(state.grid, piece);
  const full = fullRows(locked);
  let grid = locked;
  let gained = 0;
  if (full.length > 0) {
    grid = clearRows(locked, full);
    gained = lineScore(full.length, state.level);
  }
  const lines = state.lines + full.length;
  const level = nextLevel(lines);
  return spawnNext({
    ...state,
    grid,
    active: null,
    score: state.score + gained,
    lines,
    level,
    gravityMs: gravityForLevel(level),
    dropAccumulatorMs: 0,
  });
}

function movedBy(piece: ActivePiece, dRow: number, dCol: number): ActivePiece {
  return { ...piece, row: piece.row + dRow, col: piece.col + dCol };
}

/** Row the active piece would land on if hard-dropped. */
export function getGhostRow(state: GameState): number {
  if (!state.active) return 0;
  let p = state.active;
  while (!collides(state.grid, movedBy(p, 1, 0))) {
    p = movedBy(p, 1, 0);
  }
  return p.row;
}

export function reduce(state: GameState, action: Action): GameState {
  switch (action.type) {
    case 'START':
      if (state.phase !== 'welcome') return state;
      return spawnNext({ ...state, grid: emptyGrid(), phase: 'playing' });

    case 'RESTART': {
      // Keep the current RNG state rather than reseeding, so a restart is
      // still deterministic.
      const fresh: GameState = {
        grid: emptyGrid(),
        active: null,
        nextQueue: [],
        bag: [],
        rngState: state.rngState,
        score: 0,
        lines: 0,
        level: 0,
        phase: 'playing',
        gravityMs: gravityForLevel(0),
        dropAccumulatorMs: 0,
        simpleRotation: state.simpleRotation,
      };
      const q = ensureQueue(fresh.bag, fresh.rngState, fresh.nextQueue, PREVIEW);
      return spawnNext({ ...fresh, bag: q.bag, rngState: q.rngState, nextQueue: q.queue });
    }

    case 'PAUSE':
      return state.phase === 'playing' ? { ...state, phase: 'paused' } : state;

    case 'RESUME':
      return state.phase === 'paused' ? { ...state, phase: 'playing' } : state;

    default:
      break;
  }

  if (state.phase !== 'playing' || !state.active) return state;
  const active = state.active;

  switch (action.type) {
    case 'MOVE': {
      const moved = movedBy(active, 0, action.dir);
      return collides(state.grid, moved) ? state : { ...state, active: moved };
    }

    case 'SOFT_DROP': {
      const moved = movedBy(active, 1, 0);
      if (collides(state.grid, moved)) {
        // Only TICK locks pieces, so a grounded soft drop does nothing.
        return state;
      }
      return {
        ...state,
        active: moved,
        score: state.score + SOFT_DROP_POINTS,
        dropAccumulatorMs: 0,
      };
    }

    case 'HARD_DROP': {
      let dropped = 0;
      let p = active;
      while (!collides(state.grid, movedBy(p, 1, 0))) {
        p = movedBy(p, 1, 0);
        dropped++;
      }
      const scored = { ...state, score: state.score + dropped * HARD_DROP_POINTS };
      return lockActive(scored, p);
    }

    case 'ROTATE': {
      const result = tryRotate(state.grid, active, action.dir, state.simpleRotation);
      return result ? { ...state, active: result } : state;
    }

    case 'TICK': {
      let acc = state.dropAccumulatorMs + action.dtMs;
      let cur = state;
      // A long frame can owe several gravity steps.
      while (cur.active && acc >= cur.gravityMs) {
        acc -= cur.gravityMs;
        const piece = cur.active;
        const moved = movedBy(piece, 1, 0);
        if (collides(cur.grid, moved)) {
          cur = lockActive(cur, piece);
          acc = 0; // the new piece starts with a full gravity interval
          if (cur.phase !== 'playing') break;
        } else {
          cur = { ...cur, active: moved };
        }
      }
      return cur.phase === 'playing' ? { ...cur, dropAccumulatorMs: acc } : cur;
    }

    default:
      return state;
  }
}

export { cellsOf, emptyGrid } from './board';
