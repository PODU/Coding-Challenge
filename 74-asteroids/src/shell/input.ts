/**
 * Keyboard → InputState (browser-only, impure).
 *
 * Default map: ArrowLeft/A left, ArrowRight/D right, ArrowUp/W thrust,
 * Space fire, Enter start.
 */

import type { InputState } from "../types.js";

export class InputController {
  private readonly keys = new Set<string>();
  private readonly target: Window;

  constructor(target: Window) {
    this.target = target;
    target.addEventListener("keydown", this.onKeyDown);
    target.addEventListener("keyup", this.onKeyUp);
  }

  private onKeyDown = (e: KeyboardEvent): void => {
    // Stop the page from scrolling on arrows / space.
    if (
      e.code === "Space" ||
      e.code.startsWith("Arrow")
    ) {
      e.preventDefault();
    }
    this.keys.add(e.code);
  };

  private onKeyUp = (e: KeyboardEvent): void => {
    this.keys.delete(e.code);
  };

  sample(): InputState {
    const k = this.keys;
    return {
      left: k.has("ArrowLeft") || k.has("KeyA"),
      right: k.has("ArrowRight") || k.has("KeyD"),
      thrust: k.has("ArrowUp") || k.has("KeyW"),
      fire: k.has("Space"),
      start: k.has("Enter"),
    };
  }

  dispose(): void {
    this.target.removeEventListener("keydown", this.onKeyDown);
    this.target.removeEventListener("keyup", this.onKeyUp);
    this.keys.clear();
  }
}
