import type { Config } from './types.js';

/**
 * Sizes are in table units, which the renderer scales to the canvas, so only
 * their ratios matter.
 */
export const defaultConfig: Config = {
  table: { width: 800, height: 480 },
  paddle: { width: 14, height: 84, speed: 420 },
  ball: {
    radius: 9,
    baseSpeed: 360,
    maxBounceAngle: Math.PI / 3, // 60 degrees
    maxSpeed: 720,
  },
  ai: {
    speed: 300, // slower than the player, so the AI can be beaten
    deadZone: 14,
    error: 0, // raise for a sloppier AI
  },
  winningScore: 11,
  serveDelay: 0.7,
};

/** defaultConfig with some fields overridden; nested objects are merged. */
export function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    ...defaultConfig,
    ...overrides,
    table: { ...defaultConfig.table, ...overrides.table },
    paddle: { ...defaultConfig.paddle, ...overrides.paddle },
    ball: { ...defaultConfig.ball, ...overrides.ball },
    ai: { ...defaultConfig.ai, ...overrides.ai },
  };
}
