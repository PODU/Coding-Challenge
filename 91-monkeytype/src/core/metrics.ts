import type { SessionSnapshot } from './session.js';

// As in monkeytype, a "word" is 5 characters.

/** Minutes from the start to the end of the test, or to now if still running. */
export function elapsedMinutes(s: SessionSnapshot, nowMs: number): number {
  if (s.startMs === null) return 0;
  const end = s.endMs ?? nowMs;
  const ms = end - s.startMs;
  return ms <= 0 ? 0 : ms / 60000;
}

/** WPM counting correct characters only. */
export function wpm(s: SessionSnapshot, nowMs: number): number {
  const minutes = elapsedMinutes(s, nowMs);
  if (minutes <= 0) return 0;
  return s.correctChars / 5 / minutes;
}

/** WPM counting every typed character. */
export function rawWpm(s: SessionSnapshot, nowMs: number): number {
  const minutes = elapsedMinutes(s, nowMs);
  if (minutes <= 0) return 0;
  return s.typedChars / 5 / minutes;
}

/** Percentage of typed characters that are correct; 100 before any typing. */
export function accuracy(s: SessionSnapshot): number {
  if (s.typedChars === 0) return 100;
  return (s.correctChars / s.typedChars) * 100;
}
