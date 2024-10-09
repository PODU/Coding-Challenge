import { describe, expect, it } from "vitest";
import { step } from "../src/core/step.js";
import { makeRng } from "../src/core/rng.js";
import { input, makeState, stillAsteroid } from "./helpers.js";

const rng = () => makeRng(7);
const DT = 1 / 60;

describe("ship ↔ asteroid death (R10)", () => {
  it("vulnerable ship overlapping an asteroid dies and loses a life", () => {
    const ast = stillAsteroid({ tier: "large", pos: { x: 480, y: 360 } });
    const s = makeState({ asteroids: [ast] }); // ship at 480,360, invuln 0
    const { state, events } = step(s, input(), DT, rng());

    expect(state.ship.alive).toBe(false);
    expect(state.lives).toBe(2);
    expect(state.phase).toBe("ship-destroyed");
    expect(events.some((e) => e.type === "ship-destroyed")).toBe(true);
    expect(state.respawnTimer).toBeGreaterThan(0);
  });

  it("invulnerable ship overlapping an asteroid does not die (E6)", () => {
    const ast = stillAsteroid({ tier: "large", pos: { x: 480, y: 360 } });
    const s = makeState({ asteroids: [ast] });
    s.ship.invulnerable = 2;
    const { state, events } = step(s, input(), DT, rng());

    expect(state.ship.alive).toBe(true);
    expect(state.lives).toBe(3);
    expect(events.some((e) => e.type === "ship-destroyed")).toBe(false);
  });

  it("losing the last life -> game-over, no respawn (E8)", () => {
    const ast = stillAsteroid({ tier: "large", pos: { x: 480, y: 360 } });
    const s = makeState({ asteroids: [ast], lives: 1 });
    const { state, events } = step(s, input(), DT, rng());

    expect(state.lives).toBe(0);
    expect(state.phase).toBe("game-over");
    expect(events.some((e) => e.type === "game-over")).toBe(true);
  });

  it("respawns with immunity after the delay elapses (E6)", () => {
    let s = makeState({ phase: "ship-destroyed", respawnTimer: DT });
    s.ship.alive = false;
    const { state } = step(s, input(), DT, rng());
    expect(state.phase).toBe("playing");
    expect(state.ship.alive).toBe(true);
    expect(state.ship.invulnerable).toBeGreaterThan(0);
  });
});
