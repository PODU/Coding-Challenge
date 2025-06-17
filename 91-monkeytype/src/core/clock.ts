/**
 * The core never reads the clock itself; callers pass the time in. The app
 * uses RealClock and tests use FakeClock.
 */
export interface Clock {
  now(): number; // milliseconds
}

/** Uses performance.now(), which is monotonic. */
export class RealClock implements Clock {
  now(): number {
    return performance.now();
  }
}

/** A clock that only moves when told to. */
export class FakeClock implements Clock {
  private t: number;

  constructor(start = 0) {
    this.t = start;
  }

  set(ms: number): void {
    this.t = ms;
  }

  advance(deltaMs: number): void {
    this.t += deltaMs;
  }

  now(): number {
    return this.t;
  }
}
