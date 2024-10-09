import { describe, expect, it } from "vitest";
import { spawnWave, waveCount, waveSpeed } from "../src/core/wave.js";
import { step } from "../src/core/step.js";
import { makeRng } from "../src/core/rng.js";
import { mag } from "../src/core/vec2.js";
import { SPAWN_SAFE_RADIUS } from "../src/core/constants.js";
import { wrappedDist2 } from "../src/core/collisions.js";
import { input, makeState, stillAsteroid, stillBullet } from "./helpers.js";

const DT = 1 / 60;
const rng = () => makeRng(42);

describe("spawnWave (R11, E12)", () => {
  it("spawns WAVE_BASE_COUNT + (wave-1) large asteroids", () => {
    expect(spawnWave(1, 960, 720, rng(), { x: 480, y: 360 }).length).toBe(waveCount(1));
    expect(waveCount(1)).toBe(4);
    expect(waveCount(3)).toBe(6);
    const w3 = spawnWave(3, 960, 720, rng(), { x: 480, y: 360 });
    expect(w3.length).toBe(6);
    expect(w3.every((a) => a.tier === "large")).toBe(true);
  });

  it("keeps all spawns outside the ship safe radius", () => {
    const shipPos = { x: 480, y: 360 };
    const rocks = spawnWave(2, 960, 720, rng(), shipPos);
    const safe2 = SPAWN_SAFE_RADIUS * SPAWN_SAFE_RADIUS;
    for (const a of rocks) {
      expect(wrappedDist2(a.pos, shipPos, 960, 720)).toBeGreaterThanOrEqual(safe2);
    }
  });

  it("later waves drift faster", () => {
    expect(waveSpeed(2)).toBeGreaterThan(waveSpeed(1));
    const w2 = spawnWave(2, 960, 720, rng(), { x: 480, y: 360 });
    for (const a of w2) expect(mag(a.vel)).toBeCloseTo(waveSpeed(2), 6);
  });
});

describe("wave clear advances the game (R11)", () => {
  it("destroying the last asteroid increments wave and spawns the next", () => {
    const ast = stillAsteroid({ tier: "small", radius: 16, pos: { x: 300, y: 300 } });
    const s = makeState({
      asteroids: [ast],
      bullets: [stillBullet({ x: 300, y: 300 })],
    });
    s.ship.invulnerable = 10;
    s.ship.pos = { x: 10, y: 10 };

    const { state, events } = step(s, input(), DT, rng());
    expect(events.some((e) => e.type === "wave-cleared")).toBe(true);
    expect(state.wave).toBe(2);
    expect(state.asteroids.length).toBe(waveCount(2));
  });
});
