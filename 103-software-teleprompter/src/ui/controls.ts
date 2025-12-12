/**
 * Hooks the page's buttons, inputs and the space bar up to the engine and
 * renderer. Anything that changes layout (font, margins, spacing, text) asks
 * for a re-measure so maxPosition stays right.
 */

import type { ScrollEngine } from "../core/scrollEngine.js";
import type { ScriptModel } from "../core/script.js";
import type { Settings } from "../core/settings.js";
import { resolveImportUrl, fromPlainText } from "../core/loaders.js";
import { NetworkAdapter } from "./net.js";
import type { Renderer } from "./renderer.js";

export interface ControlsOptions {
  document?: Document;
  net?: NetworkAdapter;
  onError?: (message: string) => void;
  onStatus?: (message: string) => void;
  /** Called after a change that affects layout. */
  requestMeasure?: () => void;
}

export class Controls {
  private readonly engine: ScrollEngine;
  private readonly renderer: Renderer;
  private readonly script: ScriptModel;
  private readonly doc: Document;
  private readonly net: NetworkAdapter;
  private readonly onError: (m: string) => void;
  private readonly onStatus: (m: string) => void;
  private readonly requestMeasure: () => void;
  private readonly disposers: Array<() => void> = [];

  constructor(
    engine: ScrollEngine,
    renderer: Renderer,
    script: ScriptModel,
    opts: ControlsOptions = {},
  ) {
    this.engine = engine;
    this.renderer = renderer;
    this.script = script;
    this.doc = opts.document ?? document;
    this.net = opts.net ?? new NetworkAdapter();
    this.onError = opts.onError ?? ((m): void => console.error(m));
    this.onStatus = opts.onStatus ?? ((): void => {});
    this.requestMeasure = opts.requestMeasure ?? ((): void => {});
  }

  private $<T extends HTMLElement>(id: string): T | null {
    return this.doc.getElementById(id) as T | null;
  }

  private listen<K extends keyof HTMLElementEventMap>(
    el: EventTarget | null,
    type: K | string,
    handler: (ev: Event) => void,
  ): void {
    if (!el) return;
    el.addEventListener(type, handler as EventListener);
    this.disposers.push(() => el.removeEventListener(type, handler as EventListener));
  }

  /** Attach all listeners. Call once; dispose() removes them. */
  bind(): void {
    this.listen(this.$("playPause"), "click", () => this.engine.toggle());
    this.listen(this.$("reset"), "click", () => this.engine.reset());

    // Spacebar toggles play/pause, except while typing in a field.
    this.listen(this.doc, "keydown", (ev) => {
      const e = ev as KeyboardEvent;
      if (e.code !== "Space" && e.key !== " ") return;
      if (this.isEditableTarget(e.target)) return;
      e.preventDefault();
      this.engine.toggle();
    });

    this.listen(this.$("speed"), "input", (ev) => {
      const v = Number((ev.target as HTMLInputElement).value);
      this.engine.setSpeed(v);
      this.reflectSpeed();
    });

    this.listen(this.$("mirror"), "click", () => {
      this.engine.setSettings({ mirrored: !this.engine.settings.mirrored });
    });

    this.bindSettingInput("fontSize", (v) => ({ fontSizePx: Number(v) }));
    this.bindSettingInput("lineSpacing", (v) => ({ lineSpacing: Number(v) }));
    this.bindSettingInput("marginLeft", (v) => ({ marginLeftPct: Number(v) }));
    this.bindSettingInput("marginRight", (v) => ({ marginRightPct: Number(v) }));

    this.listen(this.$("fontFace"), "change", (ev) => {
      const v = (ev.target as HTMLSelectElement).value;
      this.engine.setSettings({ fontFace: v });
      this.requestMeasure();
    });

    this.listen(this.$("scriptInput"), "input", (ev) => {
      const text = (ev.target as HTMLTextAreaElement).value;
      this.script.setText(text);
      this.renderer.setScript(this.script);
      this.requestMeasure();
    });

    this.listen(this.$("markdownToggle"), "change", (ev) => {
      const on = (ev.target as HTMLInputElement).checked;
      this.renderer.setRenderMarkdown(on);
      this.renderer.setScript(this.script);
      this.requestMeasure();
    });

    this.listen(this.$("importButton"), "click", () => {
      const input = this.$<HTMLInputElement>("importUrl");
      if (input) void this.importFromUrl(input.value);
    });

    this.reflectSpeed();
  }

  private bindSettingInput(
    id: string,
    toPartial: (value: string) => Partial<Settings>,
  ): void {
    this.listen(this.$(id), "input", (ev) => {
      const v = (ev.target as HTMLInputElement).value;
      this.engine.setSettings(toPartial(v));
      this.requestMeasure();
    });
  }

  private reflectSpeed(): void {
    const out = this.$("speedValue");
    if (out) out.textContent = `${Math.round(this.engine.settings.speed)} px/s`;
  }

  private isEditableTarget(target: EventTarget | null): boolean {
    const el = target as HTMLElement | null;
    if (!el) return false;
    const tag = el.tagName;
    return (
      tag === "TEXTAREA" ||
      tag === "INPUT" ||
      tag === "SELECT" ||
      el.isContentEditable === true
    );
  }

  /**
   * Fetch a script from a URL and load it. Failures are reported through
   * onError and leave the engine alone.
   */
  async importFromUrl(rawUrl: string): Promise<void> {
    const url = rawUrl.trim();
    if (url === "") {
      this.onError("Enter a URL to import.");
      return;
    }
    const resolved = resolveImportUrl(url);
    this.onStatus(`Importing ${resolved}…`);
    try {
      const body = await this.net.fetchText(resolved);
      const text = fromPlainText(body);
      this.script.setText(text);
      this.renderer.setScript(this.script);
      const input = this.$<HTMLTextAreaElement>("scriptInput");
      if (input) input.value = text;
      this.requestMeasure();
      this.onStatus(`Imported ${text.length} characters.`);
    } catch (err) {
      this.onError(`Import failed: ${(err as Error).message}`);
    }
  }

  dispose(): void {
    for (const d of this.disposers) d();
    this.disposers.length = 0;
  }
}
