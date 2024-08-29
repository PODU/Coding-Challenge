import { easeOutCubic, type SpinPlan } from "../core/physics.js";

/**
 * Animates a spin from `fromAngle` to `plan.targetAngle` with an ease-out,
 * calling onTick every frame and onDone once at the end.
 */
export class SpinAnimator {
  private rafId: number | null = null;
  private running = false;

  get isRunning(): boolean {
    return this.running;
  }

  spin(
    plan: SpinPlan,
    fromAngle: number,
    onTick: (angle: number) => void,
    onDone: (winnerIndex: number) => void,
  ): void {
    if (this.running) return;
    this.running = true;

    const start = now();
    const delta = plan.targetAngle - fromAngle;

    const frame = (): void => {
      const elapsed = now() - start;
      const t = Math.min(1, elapsed / plan.durationMs);
      const angle = fromAngle + delta * easeOutCubic(t);
      onTick(angle);

      if (t < 1) {
        this.rafId = raf(frame);
      } else {
        onTick(plan.targetAngle);
        this.running = false;
        this.rafId = null;
        onDone(plan.winnerIndex);
      }
    };

    this.rafId = raf(frame);
  }

  cancel(): void {
    if (this.rafId !== null) caf(this.rafId);
    this.rafId = null;
    this.running = false;
  }
}

function now(): number {
  return typeof performance !== "undefined" ? performance.now() : Date.now();
}

function raf(cb: () => void): number {
  if (typeof requestAnimationFrame !== "undefined") return requestAnimationFrame(cb);
  return setTimeout(cb, 16) as unknown as number;
}

function caf(id: number): void {
  if (typeof cancelAnimationFrame !== "undefined") cancelAnimationFrame(id);
  else clearTimeout(id);
}
