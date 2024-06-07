import type { InputState } from '../core/types.js';

/** Arrows or A/D move, Space or Up fires, Enter starts or restarts. */
export class KeyboardInput {
  private readonly down = new Set<string>();
  private startPressed = false;

  constructor(target: Window = window) {
    target.addEventListener('keydown', (e) => {
      if (['ArrowLeft', 'ArrowRight', 'ArrowUp', ' ', 'Spacebar'].includes(e.key)) {
        e.preventDefault();
      }
      if (e.key === 'Enter') this.startPressed = true;
      this.down.add(e.key);
    });
    target.addEventListener('keyup', (e) => {
      this.down.delete(e.key);
    });
  }

  snapshot(): InputState {
    return {
      left: this.down.has('ArrowLeft') || this.down.has('a') || this.down.has('A'),
      right: this.down.has('ArrowRight') || this.down.has('d') || this.down.has('D'),
      fire:
        this.down.has(' ') ||
        this.down.has('Spacebar') ||
        this.down.has('ArrowUp'),
    };
  }

  /** Reports whether Enter was pressed since the last call. */
  consumeStart(): boolean {
    const v = this.startPressed;
    this.startPressed = false;
    return v;
  }
}
