import { defaultConfig, type GameConfig } from '../src/core/config.js';
import { Game } from '../src/core/game.js';
import { alienRect } from '../src/core/fleet.js';
import { Mulberry32 } from '../src/core/rng.js';
import type { GameEvent, GameState, InputState } from '../src/core/types.js';

export const NO_INPUT: InputState = { left: false, right: false, fire: false };
export const FIRE: InputState = { left: false, right: false, fire: true };

/** Shallow-merge overrides onto the default config (one level of nesting). */
export function makeConfig(overrides: Partial<GameConfig> = {}): GameConfig {
  const base = defaultConfig();
  const merged = { ...base } as GameConfig;
  for (const key of Object.keys(overrides) as (keyof GameConfig)[]) {
    const ov = overrides[key];
    const bv = base[key];
    if (ov && typeof ov === 'object' && !Array.isArray(ov) && typeof bv === 'object') {
      // @ts-expect-error structural merge of nested config objects
      merged[key] = { ...bv, ...ov };
    } else {
      // @ts-expect-error assigning override
      merged[key] = ov;
    }
  }
  return merged;
}

export function newGame(config?: GameConfig, seed = 1): Game {
  const cfg = config ?? defaultConfig();
  return new Game(cfg, new Mulberry32(seed), seed);
}

/** Access the live (mutable) state object for test setup. */
export function mutState(g: Game): GameState {
  return g.state as GameState;
}

/** Place a small player shot centred on an alien and resolve with dt=0. */
export function shootAlien(
  g: Game,
  row: number,
  col: number,
): GameEvent[] {
  const s = mutState(g);
  const r = alienRect(s.fleet, row, col);
  s.playerShot = {
    pos: { x: r.x + r.w / 2 - 1, y: r.y + r.h / 2 - 1 },
    vel: { x: 0, y: -1 },
    w: 2,
    h: 2,
  };
  return g.step(NO_INPUT, 0);
}

export function countEvents(events: GameEvent[], type: GameEvent['type']): number {
  return events.filter((e) => e.type === type).length;
}
