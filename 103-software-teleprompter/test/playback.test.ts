import { describe, it, expect } from "vitest";
import { ScrollEngine } from "../src/core/scrollEngine.js";
import { ManualClock } from "../src/core/clock.js";
import { PlaybackState } from "../src/core/playback.js";

function ready() {
  const clock = new ManualClock(0);
  const engine = new ScrollEngine(clock.clock, undefined);
  engine.setViewportHeight(500);
  engine.setContentHeight(5000); // maxPosition = 4500
  return { clock, engine };
}

describe("PlaybackState transitions", () => {
  it("starts Paused", () => {
    const { engine } = ready();
    expect(engine.state).toBe(PlaybackState.Paused);
    expect(engine.isPlaying).toBe(false);
  });

  it("play -> Playing, pause -> Paused", () => {
    const { engine } = ready();
    engine.play();
    expect(engine.state).toBe(PlaybackState.Playing);
    engine.pause();
    expect(engine.state).toBe(PlaybackState.Paused);
  });

  it("play is idempotent", () => {
    const { engine } = ready();
    engine.play();
    engine.play();
    expect(engine.state).toBe(PlaybackState.Playing);
  });

  it("toggle alternates state", () => {
    const { engine } = ready();
    engine.toggle();
    expect(engine.state).toBe(PlaybackState.Playing);
    engine.toggle();
    expect(engine.state).toBe(PlaybackState.Paused);
  });

  it("reset always returns to Paused at top", () => {
    const { clock, engine } = ready();
    engine.play();
    engine.tick(0);
    clock.set(1000);
    engine.tick(1000);
    engine.reset();
    expect(engine.state).toBe(PlaybackState.Paused);
    expect(engine.position).toBe(0);
  });
});
