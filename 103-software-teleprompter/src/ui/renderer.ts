/**
 * Draws the engine state into the DOM each frame (render) and feeds measured
 * heights back to the engine (measure).
 */

import type { ScrollEngine } from "../core/scrollEngine.js";
import type { ScriptModel } from "../core/script.js";
import { fromMarkdown } from "../core/loaders.js";

export class Renderer {
  private readonly root: HTMLElement;
  private readonly viewport: HTMLElement;
  private readonly content: HTMLElement;
  private renderMarkdown = false;

  /** `root` should contain `.tp-viewport` and `.tp-content`; missing ones are created. */
  constructor(root: HTMLElement) {
    this.root = root;
    this.viewport = this.ensureChild(".tp-viewport", root);
    this.content = this.ensureChild(".tp-content", this.viewport);
  }

  private ensureChild(selector: string, parent: HTMLElement): HTMLElement {
    const existing = parent.querySelector<HTMLElement>(selector);
    if (existing) return existing;
    const el = parent.ownerDocument.createElement("div");
    el.className = selector.replace(/^\./, "");
    parent.appendChild(el);
    return el;
  }

  get contentElement(): HTMLElement {
    return this.content;
  }

  setRenderMarkdown(on: boolean): void {
    this.renderMarkdown = on;
  }

  setScript(script: ScriptModel): void {
    if (this.renderMarkdown) {
      this.content.innerHTML = fromMarkdown(script.text).html;
    } else {
      this.content.textContent = script.text;
    }
  }

  render(engine: ScrollEngine): void {
    const s = engine.settings;

    const mirror = s.mirrored ? " scaleX(-1)" : "";
    this.content.style.transform = `translateY(${-engine.position}px)${mirror}`;
    this.content.style.transformOrigin = "center top";

    this.content.style.fontFamily = s.fontFace;
    this.content.style.fontSize = `${s.fontSizePx}px`;
    this.content.style.lineHeight = `${s.lineSpacing}`;
    this.content.style.paddingLeft = `${s.marginLeftPct}%`;
    this.content.style.paddingRight = `${s.marginRightPct}%`;

    this.root.dataset["playing"] = String(engine.isPlaying);
    this.root.dataset["mirrored"] = String(s.mirrored);
  }

  measure(engine: ScrollEngine): void {
    // Pad by one viewport height so the last line can scroll up to the reading
    // line.
    const viewportHeight = this.viewport.clientHeight;
    engine.setViewportHeight(viewportHeight);
    engine.setContentHeight(this.content.scrollHeight + viewportHeight);
  }
}
