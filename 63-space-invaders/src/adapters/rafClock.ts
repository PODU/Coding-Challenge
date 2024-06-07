/**
 * Runs onStep at a fixed timestep from requestAnimationFrame, so the game
 * plays the same at any frame rate. Catch-up steps are capped so a paused tab
 * doesn't come back to a long burst of updates.
 */
export class RafClock {
  private readonly fixedDt: number;
  private readonly maxSteps: number;
  private accumulator = 0;
  private last = 0;
  private running = false;

  constructor(
    private readonly onStep: (dt: number) => void,
    private readonly onRender: () => void,
    fixedDt = 1 / 120,
    maxSteps = 8,
  ) {
    this.fixedDt = fixedDt;
    this.maxSteps = maxSteps;
  }

  start(): void {
    if (this.running) return;
    this.running = true;
    this.last = performance.now();
    requestAnimationFrame(this.frame);
  }

  stop(): void {
    this.running = false;
  }

  private frame = (now: number): void => {
    if (!this.running) return;
    let frameTime = (now - this.last) / 1000;
    this.last = now;
    if (frameTime > 0.25) frameTime = 0.25; // clamp huge gaps

    this.accumulator += frameTime;
    let steps = 0;
    while (this.accumulator >= this.fixedDt && steps < this.maxSteps) {
      this.onStep(this.fixedDt);
      this.accumulator -= this.fixedDt;
      steps++;
    }
    if (steps >= this.maxSteps) this.accumulator = 0; // drop backlog

    this.onRender();
    requestAnimationFrame(this.frame);
  };
}
