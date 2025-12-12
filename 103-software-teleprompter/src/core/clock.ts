/**
 * A Clock returns the current time in milliseconds. The app uses
 * performance.now(); tests use ManualClock so they never have to wait.
 */

export type Clock = () => number;

export const SystemClock: Clock = () =>
  typeof performance !== "undefined" ? performance.now() : Date.now();

/** Test clock. Pass `.clock` to the engine and move time with `advance`. */
export class ManualClock {
  private t: number;

  constructor(start = 0) {
    this.t = start;
  }

  now(): number {
    return this.t;
  }

  advance(ms: number): void {
    this.t += ms;
  }

  set(ms: number): void {
    this.t = ms;
  }

  readonly clock: Clock = () => this.t;
}
