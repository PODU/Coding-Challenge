/**
 * Entry point. Connects the engine to the DOM and runs the animation loop; this
 * is the only file that uses real time and requestAnimationFrame.
 */

import { ScrollEngine } from "./core/scrollEngine.js";
import { ScriptModel } from "./core/script.js";
import { SystemClock } from "./core/clock.js";
import { DEFAULT_SETTINGS } from "./core/settings.js";
import { Renderer } from "./ui/renderer.js";
import { Controls } from "./ui/controls.js";

const SAMPLE_SCRIPT = `Welcome to the teleprompter.

Paste or type your script here, then press Play — or hit the space bar —
to start scrolling. Use the controls to set font, size, margins, line
spacing, and speed.

Toggle Mirror for a beam-splitter rig. Import a script from a URL, a
GitHub markdown file, or a Google Doc using the import box.

Look just past the camera and read naturally. Good luck.`;

function main(): void {
  const stage = document.getElementById("stage");
  if (!stage) throw new Error("Missing #stage element");

  const engine = new ScrollEngine(SystemClock, DEFAULT_SETTINGS);
  const script = new ScriptModel(SAMPLE_SCRIPT);
  const renderer = new Renderer(stage);

  const textarea = document.getElementById("scriptInput") as HTMLTextAreaElement | null;
  if (textarea) textarea.value = SAMPLE_SCRIPT;
  renderer.setScript(script);

  const status = document.getElementById("status");
  const setStatus = (m: string): void => {
    if (status) {
      status.textContent = m;
      status.dataset["kind"] = "info";
    }
  };
  const setError = (m: string): void => {
    if (status) {
      status.textContent = m;
      status.dataset["kind"] = "error";
    }
    console.error(m);
  };

  const measure = (): void => {
    renderer.measure(engine);
    renderer.render(engine);
  };

  const controls = new Controls(engine, renderer, script, {
    onError: setError,
    onStatus: setStatus,
    requestMeasure: measure,
  });
  controls.bind();

  syncInputsFromSettings(engine.settings);
  window.addEventListener("resize", measure);

  // Measure again on the next frame, once fonts and styles have settled.
  measure();
  requestAnimationFrame(measure);

  const frame = (): void => {
    engine.tick(SystemClock());
    renderer.render(engine);
    updatePlayButton(engine.isPlaying);
    requestAnimationFrame(frame);
  };
  requestAnimationFrame(frame);
}

function syncInputsFromSettings(s: {
  speed: number;
  fontSizePx: number;
  lineSpacing: number;
  marginLeftPct: number;
  marginRightPct: number;
}): void {
  setValue("speed", s.speed);
  setValue("fontSize", s.fontSizePx);
  setValue("lineSpacing", s.lineSpacing);
  setValue("marginLeft", s.marginLeftPct);
  setValue("marginRight", s.marginRightPct);
  const speedValue = document.getElementById("speedValue");
  if (speedValue) speedValue.textContent = `${Math.round(s.speed)} px/s`;
}

function setValue(id: string, value: number): void {
  const el = document.getElementById(id) as HTMLInputElement | null;
  if (el) el.value = String(value);
}

function updatePlayButton(playing: boolean): void {
  const btn = document.getElementById("playPause");
  if (btn) btn.textContent = playing ? "Pause" : "Play";
}

if (typeof document !== "undefined") {
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", main);
  } else {
    main();
  }
}
