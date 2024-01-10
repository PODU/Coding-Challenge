// Turns keyboard events into Actions. Auto-repeat (DAS/ARR) for moving and soft
// dropping lives here so the core never deals with input timing.

import type { Action } from '../core/index';

const DAS_MS = 160; // delay before auto-repeat starts
const ARR_MS = 40; // auto-repeat interval

type Dispatch = (action: Action) => void;

export class InputController {
  private repeatTimers = new Map<string, ReturnType<typeof setTimeout>>();
  private repeatIntervals = new Map<string, ReturnType<typeof setInterval>>();
  private readonly onKeyDown = (e: KeyboardEvent) => this.handleDown(e);
  private readonly onKeyUp = (e: KeyboardEvent) => this.handleUp(e);

  constructor(private readonly dispatch: Dispatch) {}

  attach(target: Window | HTMLElement = window): void {
    target.addEventListener('keydown', this.onKeyDown as EventListener);
    target.addEventListener('keyup', this.onKeyUp as EventListener);
  }

  detach(target: Window | HTMLElement = window): void {
    target.removeEventListener('keydown', this.onKeyDown as EventListener);
    target.removeEventListener('keyup', this.onKeyUp as EventListener);
    this.clearAll();
  }

  private handleDown(e: KeyboardEvent): void {
    const action = this.mapKey(e.key);
    if (!action) return;
    e.preventDefault();
    if (e.repeat) return; // we do our own auto-repeat

    this.dispatch(action);

    // Only movement and soft drop auto-repeat.
    if (action.type === 'MOVE' || action.type === 'SOFT_DROP') {
      const code = e.key;
      this.stopRepeat(code);
      const timer = setTimeout(() => {
        const interval = setInterval(() => this.dispatch(action), ARR_MS);
        this.repeatIntervals.set(code, interval);
      }, DAS_MS);
      this.repeatTimers.set(code, timer);
    }
  }

  private handleUp(e: KeyboardEvent): void {
    this.stopRepeat(e.key);
  }

  private stopRepeat(code: string): void {
    const t = this.repeatTimers.get(code);
    if (t !== undefined) {
      clearTimeout(t);
      this.repeatTimers.delete(code);
    }
    const i = this.repeatIntervals.get(code);
    if (i !== undefined) {
      clearInterval(i);
      this.repeatIntervals.delete(code);
    }
  }

  private clearAll(): void {
    for (const t of this.repeatTimers.values()) clearTimeout(t);
    for (const i of this.repeatIntervals.values()) clearInterval(i);
    this.repeatTimers.clear();
    this.repeatIntervals.clear();
  }

  private mapKey(key: string): Action | null {
    switch (key) {
      case 'ArrowLeft':
        return { type: 'MOVE', dir: -1 };
      case 'ArrowRight':
        return { type: 'MOVE', dir: 1 };
      case 'ArrowDown':
        return { type: 'SOFT_DROP' };
      case 'ArrowUp':
      case 'x':
      case 'X':
        return { type: 'ROTATE', dir: 1 };
      case 'z':
      case 'Z':
        return { type: 'ROTATE', dir: -1 };
      case ' ':
        return { type: 'HARD_DROP' };
      default:
        return null;
    }
  }
}
