import type { Rect, Vec2 } from './types.js';

export interface GameConfig {
  bounds: Rect;
  lives: number;

  ship: {
    width: number;
    height: number;
    speed: number; // px/sec
    marginBottom: number; // gap between the ship and the bottom edge
    fireCooldown: number; // seconds
  };

  shot: {
    width: number;
    height: number;
    speed: number; // px/sec
  };

  bomb: {
    width: number;
    height: number;
    speed: number; // px/sec
    interval: number; // seconds between drop attempts
    maxInFlight: number;
  };

  fleet: {
    cols: number;
    rows: number;
    cellSize: Vec2;
    gap: Vec2;
    startOrigin: Vec2; // top-left of the grid in wave 1
    baseSpeed: number; // px/sec in wave 1 with a full fleet
    stepDown: number; // how far the fleet drops at each wall
    descendSpeedup: number; // speed multiplier applied at each drop
    maxSpeed: number;
    attritionFactor: number; // speed-up as aliens are killed
    waveSpeedup: number; // extra base speed per wave
    waveDescend: number; // each wave starts this much lower
  };

  scoring: {
    rowPoints: number[]; // index 0 is the top row
    ufoPoints: number;
  };

  bunkers: {
    count: number;
    cols: number;
    rows: number;
    cellSize: Vec2;
    y: number; // top edge of the bunkers
  };

  ufo: {
    enabled: boolean;
    width: number;
    height: number;
    speed: number;
    interval: number; // seconds between appearances
    y: number;
  };

  /** If true, a bomb that meets the player's shot destroys both. */
  bombAnnihilatesShot: boolean;
}

/** 800x600 field, 5x11 fleet, 4 bunkers, classic scoring. */
export function defaultConfig(): GameConfig {
  return {
    bounds: { x: 0, y: 0, w: 800, h: 600 },
    lives: 3,

    ship: {
      width: 40,
      height: 16,
      speed: 260,
      marginBottom: 40,
      fireCooldown: 0.45,
    },

    shot: {
      width: 3,
      height: 14,
      speed: 480,
    },

    bomb: {
      width: 4,
      height: 12,
      speed: 200,
      interval: 0.9,
      maxInFlight: 3,
    },

    fleet: {
      cols: 11,
      rows: 5,
      cellSize: { x: 24, y: 16 },
      gap: { x: 12, y: 14 },
      startOrigin: { x: 208, y: 70 },
      baseSpeed: 28,
      stepDown: 18,
      descendSpeedup: 1.12,
      maxSpeed: 360,
      attritionFactor: 6,
      waveSpeedup: 0.18,
      waveDescend: 24,
    },

    scoring: {
      rowPoints: [30, 20, 20, 10, 10],
      ufoPoints: 150,
    },

    bunkers: {
      count: 4,
      cols: 8,
      rows: 6,
      cellSize: { x: 8, y: 6 },
      y: 480,
    },

    ufo: {
      enabled: true,
      width: 36,
      height: 14,
      speed: 120,
      interval: 18,
      y: 44,
    },

    bombAnnihilatesShot: false,
  };
}
