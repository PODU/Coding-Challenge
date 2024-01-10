// requestAnimationFrame loop: one TICK with the elapsed time per frame, then a render.

export class GameLoop {
  private rafId: number | null = null;
  private lastTs = 0;

  constructor(
    private readonly onFrame: (dtMs: number) => void,
  ) {}

  start(): void {
    if (this.rafId !== null) return;
    this.lastTs = performance.now();
    const frame = (ts: number) => {
      // Cap dt so returning to a background tab doesn't drop the piece at once.
      const dt = Math.min(ts - this.lastTs, 100);
      this.lastTs = ts;
      this.onFrame(dt);
      this.rafId = requestAnimationFrame(frame);
    };
    this.rafId = requestAnimationFrame(frame);
  }

  stop(): void {
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
  }
}
