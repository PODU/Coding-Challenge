/**
 * Shared core/shell contract: input, game state, events.
 */

import type { Asteroid, AsteroidTier, Bullet, Ship } from "./core/entities.js";

export type { Asteroid, AsteroidTier, Bullet, Ship };

export type InputState = {
  left: boolean; // rotate counter-clockwise
  right: boolean; // rotate clockwise
  thrust: boolean; // accelerate forward
  fire: boolean; // shoot (edge-triggered inside core via cooldown)
  start: boolean; // start / restart from attract or game-over
};

export type Phase = "attract" | "playing" | "ship-destroyed" | "game-over";

export type GameState = {
  phase: Phase;
  ship: Ship;
  bullets: Bullet[];
  asteroids: Asteroid[];
  score: number;
  lives: number;
  wave: number;
  width: number;
  height: number;
  respawnTimer: number; // s until ship respawns after death
};

export type GameEvent =
  | { type: "asteroid-hit"; tier: AsteroidTier; points: number }
  | { type: "ship-destroyed" }
  | { type: "wave-cleared"; wave: number }
  | { type: "bullet-fired" }
  | { type: "thrust" }
  | { type: "game-over" };
