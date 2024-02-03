import { describe, expect, it, vi } from 'vitest';
import {
  formatDate,
  formatTime,
  startClock,
  type Scheduler,
} from '../src/clock.js';

describe('formatTime', () => {
  it('zero-pads midnight', () => {
    expect(formatTime(new Date(2026, 5, 26, 0, 0, 0))).toBe('00:00:00');
  });

  it('zero-pads single-digit hours/minutes/seconds', () => {
    expect(formatTime(new Date(2026, 5, 26, 9, 7, 5))).toBe('09:07:05');
  });

  it('formats the last second of the day in 24h', () => {
    expect(formatTime(new Date(2026, 5, 26, 23, 59, 59))).toBe('23:59:59');
  });
});

describe('formatDate', () => {
  it('produces an exact human-readable string', () => {
    // 26 June 2026 is a Friday.
    expect(formatDate(new Date(2026, 5, 26))).toBe('Friday, 26 June 2026');
  });

  it('handles the first of January', () => {
    // 1 January 2024 is a Monday.
    expect(formatDate(new Date(2024, 0, 1))).toBe('Monday, 1 January 2024');
  });
});

describe('startClock', () => {
  /** Fake scheduler that records the pending callback without using real timers. */
  function fakeScheduler(): Scheduler & { run(): void; pending: number } {
    let cb: (() => void) | null = null;
    let nextHandle = 1;
    return {
      pending: 0,
      set(callback) {
        cb = callback;
        this.pending++;
        return nextHandle++;
      },
      clear() {
        cb = null;
      },
      run() {
        const c = cb;
        cb = null;
        if (c) c();
      },
    };
  }

  it('ticks immediately and then once per advanced second', () => {
    let seconds = 0;
    const now = (): Date => new Date(2026, 5, 26, 14, 5, seconds);
    const onTick = vi.fn();
    const sched = fakeScheduler();

    startClock(now, onTick, sched);

    // Immediate tick at second 0.
    expect(onTick).toHaveBeenCalledTimes(1);
    expect(onTick).toHaveBeenLastCalledWith('14:05:00', 'Friday, 26 June 2026');

    seconds = 1;
    sched.run();
    expect(onTick).toHaveBeenCalledTimes(2);
    expect(onTick).toHaveBeenLastCalledWith('14:05:01', 'Friday, 26 June 2026');

    seconds = 2;
    sched.run();
    expect(onTick).toHaveBeenCalledTimes(3);
    expect(onTick).toHaveBeenLastCalledWith('14:05:02', 'Friday, 26 June 2026');
  });

  it('schedules to the next whole second (drift correction)', () => {
    const now = (): Date => new Date(2026, 5, 26, 14, 5, 9, 250);
    const delays: number[] = [];
    const sched: Scheduler = {
      set: (_cb, delay) => {
        delays.push(delay);
        return 1;
      },
      clear: () => undefined,
    };

    startClock(now, vi.fn(), sched);
    expect(delays[0]).toBe(750); // 1000 - 250ms past the second
  });

  it('stops ticking after Stop is called', () => {
    let seconds = 0;
    const now = (): Date => new Date(2026, 5, 26, 14, 5, seconds);
    const onTick = vi.fn();
    const sched = fakeScheduler();

    const stop = startClock(now, onTick, sched);
    expect(onTick).toHaveBeenCalledTimes(1);

    stop();
    seconds = 1;
    sched.run(); // callback was cleared, nothing scheduled
    expect(onTick).toHaveBeenCalledTimes(1);
  });
});
