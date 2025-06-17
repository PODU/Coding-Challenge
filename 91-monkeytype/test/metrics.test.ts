import { describe, it, expect } from 'vitest';
import { wpm, rawWpm, accuracy, elapsedMinutes } from '../src/core/metrics.js';
import type { SessionSnapshot } from '../src/core/session.js';

/** Build a minimal snapshot for metric math; fields not under test are filler. */
function snap(p: Partial<SessionSnapshot>): SessionSnapshot {
  return {
    status: 'running',
    target: '',
    statuses: [],
    extras: [],
    cursor: 0,
    typedChars: 0,
    correctChars: 0,
    incorrectChars: 0,
    startMs: 0,
    endMs: null,
    durationMs: 60000,
    ...p,
  };
}

describe('metrics — WPM / raw / accuracy', () => {
  it('canonical WPM: 100 correct chars over 1 minute => 20', () => {
    const s = snap({ correctChars: 100, typedChars: 100, startMs: 0 });
    expect(wpm(s, 60000)).toBe(20);
  });

  it('100 correct chars over 30 s => 40', () => {
    const s = snap({ correctChars: 100, typedChars: 100, startMs: 0 });
    expect(wpm(s, 30000)).toBe(40);
  });

  it('raw vs net: correct=90 typed=100 over 1 min => wpm 18, raw 20', () => {
    const s = snap({ correctChars: 90, typedChars: 100, startMs: 0 });
    expect(wpm(s, 60000)).toBe(18);
    expect(rawWpm(s, 60000)).toBe(20);
  });

  it('accuracy: 90/100 => 90, 45/50 => 90', () => {
    expect(accuracy(snap({ correctChars: 90, typedChars: 100 }))).toBe(90);
    expect(accuracy(snap({ correctChars: 45, typedChars: 50 }))).toBe(90);
  });

  it('zero-time guard: start === now => 0, no NaN/Infinity', () => {
    const s = snap({ correctChars: 50, typedChars: 50, startMs: 1000 });
    expect(wpm(s, 1000)).toBe(0);
    expect(rawWpm(s, 1000)).toBe(0);
    expect(Number.isFinite(wpm(s, 1000))).toBe(true);
  });

  it('zero-keystroke guard: typedChars === 0 => accuracy 100', () => {
    expect(accuracy(snap({ typedChars: 0, correctChars: 0 }))).toBe(100);
  });

  it('frozen after finish: uses endMs, advancing now does not change wpm', () => {
    const s = snap({ correctChars: 100, typedChars: 100, startMs: 0, endMs: 60000 });
    expect(wpm(s, 60000)).toBe(20);
    expect(wpm(s, 120000)).toBe(20); // advancing now past endMs is ignored
  });

  it('elapsedMinutes is 0 before start', () => {
    expect(elapsedMinutes(snap({ startMs: null }), 5000)).toBe(0);
  });
});
