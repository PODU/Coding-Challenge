import { describe, it, expect } from 'vitest';
import { TypingSession } from '../src/core/session.js';
import { wpm, accuracy } from '../src/core/metrics.js';
import type { TestConfig } from '../src/core/config.js';

const CONFIG: TestConfig = { durationSec: 30, wordCount: 25, strictBackspace: false };

/** Type a whole string char-by-char with a monotonic fake clock. */
function typeAll(s: TypingSession, text: string, startMs = 0, stepMs = 0): void {
  for (let i = 0; i < text.length; i += 1) {
    s.typeChar(text[i], startMs + i * stepMs);
  }
}

function assertInvariants(s: TypingSession): void {
  const snap = s.getState();
  expect(snap.statuses.length).toBe(snap.target.length);
  expect(snap.cursor).toBeGreaterThanOrEqual(0);
  expect(snap.cursor).toBeLessThanOrEqual(snap.target.length + snap.extras.length);
  expect(snap.typedChars).toBe(snap.correctChars + snap.incorrectChars);
}

describe('session — character correctness', () => {
  it('typing the exact target marks every char correct', () => {
    const target = 'the time person';
    const s = new TypingSession(target, CONFIG);
    typeAll(s, target);
    const snap = s.getState();
    expect(snap.statuses.every((st) => st === 'correct')).toBe(true);
    expect(snap.correctChars).toBe(target.length);
    expect(snap.incorrectChars).toBe(0);
    expect(snap.cursor).toBe(target.length);
    assertInvariants(s);
  });

  it('a wrong char at index k is incorrect and does not touch correctChars', () => {
    const target = 'hello';
    const s = new TypingSession(target, CONFIG);
    s.typeChar('h', 0);
    s.typeChar('x', 1); // wrong at index 1
    s.typeChar('l', 2);
    const snap = s.getState();
    expect(snap.statuses[1]).toBe('incorrect');
    expect(snap.incorrectChars).toBe(1);
    expect(snap.correctChars).toBe(2);
    expect(snap.cursor).toBe(3);
    assertInvariants(s);
  });

  it('idle -> running on first keystroke, records startMs', () => {
    const s = new TypingSession('abc', CONFIG);
    expect(s.getState().status).toBe('idle');
    s.typeChar('a', 1234);
    expect(s.getState().status).toBe('running');
    expect(s.getState().startMs).toBe(1234);
  });

  it('extra chars past target end land in extras, raise incorrect, never correct', () => {
    const target = 'hi';
    const s = new TypingSession(target, CONFIG);
    typeAll(s, 'hiox'); // two extra chars
    const snap = s.getState();
    expect(snap.extras).toEqual(['o', 'x']);
    expect(snap.correctChars).toBe(2);
    expect(snap.incorrectChars).toBe(2);
    expect(snap.typedChars).toBe(4);
    expect(accuracy(snap)).toBe(50);
    assertInvariants(s);
  });

  it('space mid-word counts as incorrect substitution', () => {
    const target = 'abc';
    const s = new TypingSession(target, CONFIG);
    s.typeChar('a', 0);
    s.typeChar(' ', 1); // expected 'b'
    const snap = s.getState();
    expect(snap.statuses[1]).toBe('incorrect');
    expect(snap.incorrectChars).toBe(1);
  });

  it('mixed scripted sequence yields exact counters, wpm and accuracy', () => {
    // target length 10; type 9 chars in 30s, 8 correct + 1 wrong.
    const target = 'the world '; // 10 chars incl trailing space
    const s = new TypingSession(target, CONFIG);
    typeAll(s, 'the worxd'); // 9 chars, index 7 wrong ('x' vs 'l')
    const snap = s.getState();
    expect(snap.typedChars).toBe(9);
    expect(snap.correctChars).toBe(8);
    expect(snap.incorrectChars).toBe(1);
    // 8 correct / 5 = 1.6 words / 0.5 min = 3.2 wpm
    expect(wpm(snap, 30000)).toBe(3.2);
    expect(accuracy(snap)).toBeCloseTo((8 / 9) * 100, 6);
    assertInvariants(s);
  });

  it('reset returns to idle with cleared counters', () => {
    const s = new TypingSession('abc', CONFIG);
    typeAll(s, 'abc');
    s.reset('xy');
    const snap = s.getState();
    expect(snap.status).toBe('idle');
    expect(snap.target).toBe('xy');
    expect(snap.cursor).toBe(0);
    expect(snap.typedChars).toBe(0);
    expect(snap.statuses).toEqual(['pending', 'pending']);
  });

  it('append extends the target without disturbing progress', () => {
    const s = new TypingSession('ab', CONFIG);
    typeAll(s, 'ab');
    s.append(' cd');
    const snap = s.getState();
    expect(snap.target).toBe('ab cd');
    expect(snap.cursor).toBe(2);
    expect(snap.correctChars).toBe(2);
    expect(snap.statuses.length).toBe(5);
  });
});
