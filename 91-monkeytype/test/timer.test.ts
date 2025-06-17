import { describe, it, expect } from 'vitest';
import { TypingSession } from '../src/core/session.js';
import { FakeClock } from '../src/core/clock.js';
import { wpm } from '../src/core/metrics.js';
import type { TestConfig } from '../src/core/config.js';

const CONFIG: TestConfig = { durationSec: 30, wordCount: 25, strictBackspace: false };

describe('timer — timed end with injected clock', () => {
  it('still running at 29999ms, finished exactly at 30000ms', () => {
    const clock = new FakeClock(0);
    const s = new TypingSession('the world today now here', CONFIG);
    s.typeChar('t', clock.now()); // start at t=0

    clock.set(29999);
    s.tick(clock.now());
    expect(s.getState().status).toBe('running');

    clock.set(30000);
    s.tick(clock.now());
    const snap = s.getState();
    expect(snap.status).toBe('finished');
    expect(snap.endMs).toBe(30000);
  });

  it('after finished, typeChar and backspace are ignored; counters frozen', () => {
    const clock = new FakeClock(0);
    const s = new TypingSession('the world', CONFIG);
    s.typeChar('t', clock.now());
    s.typeChar('h', clock.now());
    const before = s.getState();

    clock.set(30000);
    s.tick(clock.now());
    expect(s.getState().status).toBe('finished');

    s.typeChar('e', 31000);
    s.backspace(31000);
    const after = s.getState();
    expect(after.typedChars).toBe(before.typedChars);
    expect(after.correctChars).toBe(before.correctChars);
    expect(after.cursor).toBe(before.cursor);
  });

  it('wpm at finish uses endMs; advancing the clock afterwards leaves it unchanged', () => {
    const clock = new FakeClock(0);
    const s = new TypingSession('aaaaaaaaaa aaaaaaaaaa aaaaaaaaaa', CONFIG);
    // 10 correct chars typed instantly at t=0.
    for (let i = 0; i < 10; i += 1) s.typeChar('a', 0);

    clock.set(30000);
    s.tick(clock.now());
    const snap = s.getState();
    expect(snap.status).toBe('finished');
    // 10 correct / 5 / 0.5 min = 4 wpm.
    expect(wpm(snap, snap.endMs ?? 0)).toBe(4);
    // Advancing now well past endMs must not change it.
    expect(wpm(snap, 999999)).toBe(4);
  });

  it('tick before any keystroke is a no-op (stays idle)', () => {
    const s = new TypingSession('abc', CONFIG);
    s.tick(999999);
    expect(s.getState().status).toBe('idle');
  });
});
