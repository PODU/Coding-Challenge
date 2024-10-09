import { describe, expect, it } from "vitest";
import { createInitialState, step } from "../src/core/step.js";
import { makeRng } from "../src/core/rng.js";
import type { GameState, InputState } from "../src/types.js";

const DT = 1 / 60;

/** A reproducible scripted input log: start, then thrust/turn/fire patterns. */
function scriptedInputs(n: number): InputState[] {
  const seq: InputState[] = [];
  for (let i = 0; i < n; i++) {
    seq.push({
      start: i === 0,
      left: i % 7 === 0,
      right: i % 5 === 0,
      thrust: i % 3 === 0,
      fire: i % 4 === 0,
    });
  }
  return seq;
}

/** Replay the same seed + input log from scratch. */
function run(seed: number, inputs: InputState[]): GameState {
  const rng = makeRng(seed);
  let state = createInitialState(rng);
  for (const inp of inputs) {
    state = step(state, inp, DT, rng).state;
  }
  return state;
}

describe("determinism (NF1)", () => {
  const inputs = scriptedInputs(600);

  it("same seed + same inputs -> byte-identical final state", () => {
    const a = run(20260627, inputs);
    const b = run(20260627, inputs);
    expect(b).toEqual(a);
  });

  it("score, lives and wave replay identically", () => {
    const a = run(555, inputs);
    const b = run(555, inputs);
    expect({ score: b.score, lives: b.lives, wave: b.wave }).toEqual({
      score: a.score,
      lives: a.lives,
      wave: a.wave,
    });
  });

  it("different seeds diverge (RNG is actually used)", () => {
    const a = run(1, inputs);
    const b = run(2, inputs);
    expect(b).not.toEqual(a);
  });

  it("step does not mutate the caller's state object", () => {
    const rng = makeRng(9);
    const state = createInitialState(rng);
    const before = JSON.parse(JSON.stringify(state));
    step(state, scriptedInputs(1)[0]!, DT, rng);
    expect(state).toEqual(before);
  });
});
