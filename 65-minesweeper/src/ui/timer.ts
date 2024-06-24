/** Game clock. Only displayed; the engine never looks at it. */
export class Timer {
  private startMs: number | null = null;

  /** Does nothing if already running. */
  start(now: number = Date.now()): void {
    if (this.startMs === null) this.startMs = now;
  }

  reset(): void {
    this.startMs = null;
  }

  /** Whole seconds since start, or 0 if not started. */
  elapsedSeconds(now: number = Date.now()): number {
    if (this.startMs === null) return 0;
    return Math.floor((now - this.startMs) / 1000);
  }
}
