import { describe, it, expect, beforeEach } from "vitest";
import { ScrollEngine } from "../src/core/scrollEngine.js";
import { ManualClock } from "../src/core/clock.js";
import { PlaybackState } from "../src/core/playback.js";
import { DEFAULT_SETTINGS } from "../src/core/settings.js";

const EPS = 1e-6;

/** Build an engine with a manual clock and a generous scroll range. */
function makeEngine(opts?: { speed?: number; maxPosition?: number }) {
  const clock = new ManualClock(0);
  const engine = new ScrollEngine(clock.clock, {
    ...DEFAULT_SETTINGS,
    speed: opts?.speed ?? 100,
  });
  const max = opts?.maxPosition ?? 100_000;
  engine.setViewportHeight(1000);
  engine.setContentHeight(1000 + max); // maxPosition = max
  return { clock, engine };
}

describe("ScrollEngine", () => {
  let clock: ManualClock;
  let engine: ScrollEngine;

  beforeEach(() => {
    ({ clock, engine } = makeEngine({ speed: 100 }));
  });

  it("advances position by speed * dt over time (test plan 1)", () => {
    engine.play();
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    expect(engine.position).toBeCloseTo(100, 6);

    clock.set(1500);
    engine.tick(1500);
    expect(engine.position).toBeCloseTo(150, 6);
  });

  it("is frame-rate independent: one big tick == many small ticks (test plan 2)", () => {
    // One 1000ms tick.
    const big = makeEngine({ speed: 100 });
    big.engine.play();
    big.engine.tick(0);
    big.clock.set(1000);
    big.engine.tick(1000);

    // Sixty ~16.67ms ticks summing to 1000ms.
    const small = makeEngine({ speed: 100 });
    small.engine.play();
    small.engine.tick(0);
    let t = 0;
    for (let i = 1; i <= 60; i++) {
      t = (1000 * i) / 60;
      small.clock.set(t);
      small.engine.tick(t);
    }

    expect(big.engine.position).toBeCloseTo(100, 6);
    expect(small.engine.position).toBeCloseTo(100, 6);
    expect(Math.abs(big.engine.position - small.engine.position)).toBeLessThan(EPS);
  });

  it("does not advance while paused; advances after play (test plan 3)", () => {
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    expect(engine.position).toBe(0); // still paused

    engine.play();
    clock.set(2000);
    engine.tick(2000);
    expect(engine.position).toBeCloseTo(100, 6);
  });

  it("pause freezes and resume does not jump (edge case 10)", () => {
    engine.play();
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    expect(engine.position).toBeCloseTo(100, 6);

    engine.pause();
    clock.set(5000); // long pause
    engine.tick(5000);
    expect(engine.position).toBeCloseTo(100, 6); // frozen, no drift

    engine.play();
    clock.set(6000);
    engine.tick(6000);
    expect(engine.position).toBeCloseTo(200, 6); // only +1s integrated, no jump
  });

  it("toggle flips Paused <-> Playing (test plan 4)", () => {
    expect(engine.state).toBe(PlaybackState.Paused);
    engine.toggle();
    expect(engine.state).toBe(PlaybackState.Playing);
    engine.tick(0);
    clock.set(500);
    engine.tick(500);
    expect(engine.position).toBeCloseTo(50, 6);
    engine.toggle();
    expect(engine.state).toBe(PlaybackState.Paused);
    clock.set(1500);
    engine.tick(1500);
    expect(engine.position).toBeCloseTo(50, 6);
  });

  it("speed change mid-scroll is continuous (test plan 5)", () => {
    engine.play();
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    expect(engine.position).toBeCloseTo(100, 6);

    engine.setSpeed(200);
    clock.set(2000);
    engine.tick(2000);
    expect(engine.position).toBeCloseTo(300, 6); // 100 + 200*1, no discontinuity
  });

  it("clamps at end and auto-pauses (test plan 6)", () => {
    ({ clock, engine } = makeEngine({ speed: 100, maxPosition: 50 }));
    engine.play();
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    expect(engine.position).toBeCloseTo(50, 6);
    expect(engine.atEnd).toBe(true);
    expect(engine.state).toBe(PlaybackState.Paused);
  });

  it("play() is a no-op at the end until reset (test plan 7)", () => {
    ({ clock, engine } = makeEngine({ speed: 100, maxPosition: 50 }));
    engine.play();
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    expect(engine.atEnd).toBe(true);

    engine.play();
    expect(engine.state).toBe(PlaybackState.Paused); // still parked

    engine.reset();
    expect(engine.position).toBe(0);
    expect(engine.state).toBe(PlaybackState.Paused);
    engine.play();
    expect(engine.state).toBe(PlaybackState.Playing);
  });

  it("reset returns to top and re-enables play (test plan 8)", () => {
    engine.play();
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    expect(engine.position).toBeGreaterThan(0);
    engine.reset();
    expect(engine.position).toBe(0);
    expect(engine.state).toBe(PlaybackState.Paused);
  });

  it("re-clamps position when content shrinks (test plan 9)", () => {
    engine.seek(80);
    expect(engine.position).toBe(80);
    engine.setContentHeight(1000 + 40); // maxPosition = 40
    expect(engine.position).toBe(40);
  });

  it("speed of zero freezes; negative clamps to min (test plan 10)", () => {
    engine.play();
    engine.setSpeed(0);
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    expect(engine.position).toBe(0); // frozen

    engine.setSpeed(-50);
    expect(engine.settings.speed).toBe(0); // clamped to SPEED_MIN
  });

  it("empty/short content: maxPosition 0, atEnd immediately (edge cases 6,7)", () => {
    const c = new ManualClock(0);
    const e = new ScrollEngine(c.clock);
    e.setViewportHeight(1000);
    e.setContentHeight(500); // shorter than viewport
    expect(e.maxPosition).toBe(0);
    expect(e.atEnd).toBe(true);
    e.play();
    expect(e.state).toBe(PlaybackState.Paused); // no-op
  });

  it("absorbs a large frame gap in one clamped step (edge case 9)", () => {
    ({ clock, engine } = makeEngine({ speed: 100, maxPosition: 50 }));
    engine.play();
    engine.tick(0);
    clock.set(10_000); // 10s stall
    engine.tick(10_000);
    expect(engine.position).toBe(50); // clamped, no runaway
    expect(engine.atEnd).toBe(true);
  });

  it("ignores backwards/non-advancing clock", () => {
    engine.play();
    engine.tick(0); // baseline
    clock.set(1000);
    engine.tick(1000);
    expect(engine.position).toBeCloseTo(100, 6);
    // A backwards tick must not subtract from position.
    clock.set(500);
    engine.tick(500);
    expect(engine.position).toBeCloseTo(100, 6);
  });

  it("reports progress in [0,1]", () => {
    ({ clock, engine } = makeEngine({ speed: 100, maxPosition: 100 }));
    expect(engine.progress).toBe(0);
    engine.play();
    engine.tick(0);
    clock.set(500);
    engine.tick(500);
    expect(engine.progress).toBeCloseTo(0.5, 6);
  });
});
