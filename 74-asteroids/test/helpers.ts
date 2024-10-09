import { makeShip, type Asteroid, type Bullet } from "../src/core/entities.js";
import type { GameState, InputState, Phase } from "../src/types.js";

export const NONE: InputState = {
  left: false,
  right: false,
  thrust: false,
  fire: false,
  start: false,
};

export const input = (over: Partial<InputState> = {}): InputState => ({
  ...NONE,
  ...over,
});

/** Build a deterministic playing-state for tests. */
export function makeState(over: Partial<GameState> = {}): GameState {
  const ship = makeShip({ x: 480, y: 360 });
  ship.invulnerable = 0;
  return {
    phase: "playing" as Phase,
    ship,
    bullets: [],
    asteroids: [],
    score: 0,
    lives: 3,
    wave: 1,
    width: 960,
    height: 720,
    respawnTimer: 0,
    ...over,
  };
}

export const stillAsteroid = (a: Partial<Asteroid> & Pick<Asteroid, "tier">): Asteroid => ({
  pos: { x: 100, y: 100 },
  vel: { x: 0, y: 0 },
  radius: 60,
  angle: 0,
  spin: 0,
  shape: [1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1],
  ...a,
});

export const stillBullet = (pos: { x: number; y: number }): Bullet => ({
  pos: { x: pos.x, y: pos.y },
  vel: { x: 0, y: 0 },
  ttl: 1,
});
