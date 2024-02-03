/**
 * Clock formatting and a once-a-second tick loop. Time and timers are passed
 * in so tests can control them.
 */

export type NowFn = () => Date;

export type TickFn = (time: string, date: string) => void;

/** setTimeout/clearTimeout in a form tests can replace. */
export interface Scheduler {
  set(callback: () => void, delayMs: number): number;
  clear(handle: number): void;
}

/** Stops a running clock. Safe to call more than once. */
export type Stop = () => void;

const WEEKDAYS = [
  'Sunday',
  'Monday',
  'Tuesday',
  'Wednesday',
  'Thursday',
  'Friday',
  'Saturday',
] as const;

const MONTHS = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
] as const;

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

/** Local 24-hour time, e.g. `"14:05:09"`. */
export function formatTime(d: Date): string {
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`;
}

/**
 * e.g. `"Friday, 26 June 2026"`. Uses our own name tables rather than
 * toLocaleDateString so the output doesn't depend on the locale.
 */
export function formatDate(d: Date): string {
  const weekday = WEEKDAYS[d.getDay()];
  const month = MONTHS[d.getMonth()];
  return `${weekday}, ${d.getDate()} ${month} ${d.getFullYear()}`;
}

export const realScheduler: Scheduler = {
  set: (callback, delayMs) => setTimeout(callback, delayMs) as unknown as number,
  clear: (handle) => clearTimeout(handle),
};

/**
 * Calls `onTick` now and then at the start of every following second. Each
 * tick is scheduled against the wall clock instead of a fixed 1000ms interval,
 * so the display doesn't drift.
 */
export function startClock(
  now: NowFn,
  onTick: TickFn,
  scheduler: Scheduler = realScheduler,
): Stop {
  let handle: number | null = null;
  let stopped = false;

  const tick = (): void => {
    if (stopped) return;
    const d = now();
    onTick(formatTime(d), formatDate(d));
    const msUntilNextSecond = 1000 - (d.getTime() % 1000);
    handle = scheduler.set(tick, msUntilNextSecond);
  };

  tick();

  return () => {
    stopped = true;
    if (handle !== null) {
      scheduler.clear(handle);
      handle = null;
    }
  };
}
