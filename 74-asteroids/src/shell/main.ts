/**
 * Browser bootstrap + fixed-timestep game loop (impure shell).
 *
 * Wires the seeded core to the canvas renderer and keyboard input. The
 * simulation only ever advances by the constant DT; rendering is rAF-driven
 * via an accumulator.
 */

import { DT, FIELD_HEIGHT, FIELD_WIDTH } from "../core/constants.js";
import { makeRng } from "../core/rng.js";
import { createInitialState, step } from "../core/step.js";
import { CanvasRenderer } from "./renderer.js";
import { InputController } from "./input.js";
import type { GameState } from "../types.js";

const MAX_FRAME = 0.25; // clamp huge frame gaps (tab unfocused)

function boot(): void {
  const canvas = document.getElementById("game") as HTMLCanvasElement | null;
  if (!canvas) throw new Error("missing #game canvas");
  canvas.width = FIELD_WIDTH;
  canvas.height = FIELD_HEIGHT;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("no 2d context");

  const rng = makeRng(Date.now() >>> 0);
  const input = new InputController(window);
  const renderer = new CanvasRenderer(ctx);

  let state: GameState = createInitialState(rng, FIELD_WIDTH, FIELD_HEIGHT);
  let last = performance.now();
  let acc = 0;

  function frame(now: number): void {
    let frameTime = (now - last) / 1000;
    if (frameTime > MAX_FRAME) frameTime = MAX_FRAME;
    last = now;
    acc += frameTime;

    while (acc >= DT) {
      const snapshot = input.sample();
      const result = step(state, snapshot, DT, rng);
      state = result.state;
      acc -= DT;
    }

    renderer.draw(state);
    requestAnimationFrame(frame);
  }

  requestAnimationFrame(frame);
}

boot();
