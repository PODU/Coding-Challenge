// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from "vitest";
import { ScrollEngine } from "../src/core/scrollEngine.js";
import { ScriptModel } from "../src/core/script.js";
import { ManualClock } from "../src/core/clock.js";
import { Renderer } from "../src/ui/renderer.js";
import { Controls } from "../src/ui/controls.js";
import { NetworkAdapter, type FetchLike } from "../src/ui/net.js";
import { PlaybackState } from "../src/core/playback.js";

function buildDom(): HTMLElement {
  document.body.innerHTML = `
    <main id="stage">
      <div class="tp-viewport"><div class="tp-content"></div></div>
    </main>
    <button id="playPause"></button>
    <button id="reset"></button>
    <button id="mirror"></button>
    <input id="speed" type="range" min="0" max="400" />
    <span id="speedValue"></span>
    <select id="fontFace"><option value="Georgia, serif">Serif</option></select>
    <input id="fontSize" type="range" />
    <input id="lineSpacing" type="range" />
    <input id="marginLeft" type="range" />
    <input id="marginRight" type="range" />
    <input id="markdownToggle" type="checkbox" />
    <textarea id="scriptInput"></textarea>
    <input id="importUrl" type="text" />
    <button id="importButton"></button>
  `;
  return document.getElementById("stage") as HTMLElement;
}

describe("Renderer (jsdom smoke)", () => {
  let stage: HTMLElement;
  let engine: ScrollEngine;
  let renderer: Renderer;

  beforeEach(() => {
    stage = buildDom();
    engine = new ScrollEngine(new ManualClock(0).clock);
    renderer = new Renderer(stage);
  });

  it("applies translateY for the scroll position", () => {
    engine.setViewportHeight(100);
    engine.setContentHeight(1000);
    engine.seek(120);
    renderer.render(engine);
    const content = stage.querySelector(".tp-content") as HTMLElement;
    expect(content.style.transform).toContain("translateY(-120px)");
    expect(content.style.transform).not.toContain("scaleX");
  });

  it("adds scaleX(-1) when mirrored", () => {
    engine.setSettings({ mirrored: true });
    renderer.render(engine);
    const content = stage.querySelector(".tp-content") as HTMLElement;
    expect(content.style.transform).toContain("scaleX(-1)");
  });

  it("applies typography settings to the content element", () => {
    engine.setSettings({ fontSizePx: 72, lineSpacing: 2, marginLeftPct: 10 });
    renderer.render(engine);
    const content = stage.querySelector(".tp-content") as HTMLElement;
    expect(content.style.fontSize).toBe("72px");
    expect(content.style.lineHeight).toBe("2");
    expect(content.style.paddingLeft).toBe("10%");
  });

  it("sets text content from the script", () => {
    const script = new ScriptModel("Hello world");
    renderer.setScript(script);
    const content = stage.querySelector(".tp-content") as HTMLElement;
    expect(content.textContent).toBe("Hello world");
  });
});

describe("Controls (jsdom smoke)", () => {
  let stage: HTMLElement;
  let engine: ScrollEngine;
  let renderer: Renderer;
  let script: ScriptModel;

  beforeEach(() => {
    stage = buildDom();
    engine = new ScrollEngine(new ManualClock(0).clock);
    engine.setViewportHeight(100);
    engine.setContentHeight(5000);
    renderer = new Renderer(stage);
    script = new ScriptModel("");
  });

  it("spacebar keydown toggles playback", () => {
    new Controls(engine, renderer, script, { document }).bind();
    expect(engine.state).toBe(PlaybackState.Paused);
    document.dispatchEvent(new KeyboardEvent("keydown", { code: "Space", key: " " }));
    expect(engine.state).toBe(PlaybackState.Playing);
    document.dispatchEvent(new KeyboardEvent("keydown", { code: "Space", key: " " }));
    expect(engine.state).toBe(PlaybackState.Paused);
  });

  it("spacebar is ignored while typing in the textarea", () => {
    new Controls(engine, renderer, script, { document }).bind();
    const ta = document.getElementById("scriptInput") as HTMLTextAreaElement;
    const ev = new KeyboardEvent("keydown", { code: "Space", key: " ", bubbles: true });
    Object.defineProperty(ev, "target", { value: ta });
    document.dispatchEvent(ev);
    expect(engine.state).toBe(PlaybackState.Paused);
  });

  it("play/pause button toggles playback", () => {
    new Controls(engine, renderer, script, { document }).bind();
    (document.getElementById("playPause") as HTMLButtonElement).click();
    expect(engine.isPlaying).toBe(true);
  });

  it("speed slider input calls setSpeed", () => {
    new Controls(engine, renderer, script, { document }).bind();
    const speed = document.getElementById("speed") as HTMLInputElement;
    speed.value = "250";
    speed.dispatchEvent(new Event("input"));
    expect(engine.settings.speed).toBe(250);
    expect(document.getElementById("speedValue")?.textContent).toContain("250");
  });

  it("mirror button flips the mirrored setting", () => {
    new Controls(engine, renderer, script, { document }).bind();
    expect(engine.settings.mirrored).toBe(false);
    (document.getElementById("mirror") as HTMLButtonElement).click();
    expect(engine.settings.mirrored).toBe(true);
  });

  it("font-size input updates settings and requests a measure", () => {
    let measured = 0;
    new Controls(engine, renderer, script, {
      document,
      requestMeasure: () => measured++,
    }).bind();
    const fs = document.getElementById("fontSize") as HTMLInputElement;
    fs.value = "90";
    fs.dispatchEvent(new Event("input"));
    expect(engine.settings.fontSizePx).toBe(90);
    expect(measured).toBeGreaterThan(0);
  });

  it("script textarea input updates the model and re-measures", () => {
    let measured = 0;
    new Controls(engine, renderer, script, {
      document,
      requestMeasure: () => measured++,
    }).bind();
    const ta = document.getElementById("scriptInput") as HTMLTextAreaElement;
    ta.value = "New script body";
    ta.dispatchEvent(new Event("input"));
    expect(script.text).toBe("New script body");
    expect(measured).toBe(1);
  });

  it("import fetches via a mocked NetworkAdapter and loads the text", async () => {
    const mockFetch: FetchLike = async (url: string) => ({
      ok: true,
      status: 200,
      text: async () => `fetched from ${url}`,
    });
    const net = new NetworkAdapter(mockFetch);
    const controls = new Controls(engine, renderer, script, { document, net });
    controls.bind();

    await controls.importFromUrl("https://github.com/o/r/blob/main/s.md");
    // URL was resolved to the raw host before fetching.
    expect(script.text).toBe(
      "fetched from https://raw.githubusercontent.com/o/r/main/s.md",
    );
    const ta = document.getElementById("scriptInput") as HTMLTextAreaElement;
    expect(ta.value).toContain("fetched from");
  });

  it("import failure surfaces an error and leaves engine state intact", async () => {
    const mockFetch: FetchLike = async () => ({
      ok: false,
      status: 404,
      text: async () => "",
    });
    const net = new NetworkAdapter(mockFetch);
    const errors: string[] = [];
    const controls = new Controls(engine, renderer, script, {
      document,
      net,
      onError: (m) => errors.push(m),
    });
    controls.bind();

    await controls.importFromUrl("https://example.com/missing.txt");
    expect(errors.length).toBe(1);
    expect(errors[0]).toContain("404");
    expect(engine.state).toBe(PlaybackState.Paused);
  });
});
