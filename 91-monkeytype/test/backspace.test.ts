import { describe, it, expect } from 'vitest';
import { TypingSession } from '../src/core/session.js';
import { accuracy } from '../src/core/metrics.js';
import type { TestConfig } from '../src/core/config.js';

const RELAXED: TestConfig = { durationSec: 30, wordCount: 25, strictBackspace: false };
const STRICT: TestConfig = { durationSec: 30, wordCount: 25, strictBackspace: true };

describe('backspace', () => {
  it('after a correct char: index pending, correctChars and typedChars decremented', () => {
    const s = new TypingSession('abc', RELAXED);
    s.typeChar('a', 0);
    s.typeChar('b', 1);
    s.backspace(2);
    const snap = s.getState();
    expect(snap.cursor).toBe(1);
    expect(snap.statuses[1]).toBe('pending');
    expect(snap.correctChars).toBe(1);
    expect(snap.typedChars).toBe(1);
  });

  it('after an incorrect char: incorrectChars decremented, status pending', () => {
    const s = new TypingSession('abc', RELAXED);
    s.typeChar('a', 0);
    s.typeChar('x', 1); // wrong
    s.backspace(2);
    const snap = s.getState();
    expect(snap.incorrectChars).toBe(0);
    expect(snap.statuses[1]).toBe('pending');
    expect(snap.typedChars).toBe(1);
  });

  it('at cursor === 0 is a no-op', () => {
    const s = new TypingSession('abc', RELAXED);
    s.backspace(0);
    const snap = s.getState();
    expect(snap.cursor).toBe(0);
    expect(snap.typedChars).toBe(0);
    expect(snap.status).toBe('idle');
  });

  it('removes an extra/overflow char', () => {
    const s = new TypingSession('hi', RELAXED);
    s.typeChar('h', 0);
    s.typeChar('i', 1);
    s.typeChar('x', 2); // extra
    expect(s.getState().extras).toEqual(['x']);
    s.backspace(3);
    const snap = s.getState();
    expect(snap.extras).toEqual([]);
    expect(snap.incorrectChars).toBe(0);
    expect(snap.cursor).toBe(2);
  });

  it('relaxed mode: can backspace across a word boundary (space)', () => {
    const s = new TypingSession('ab cd', RELAXED);
    for (const ch of 'ab c') s.typeChar(ch, 0);
    expect(s.getState().cursor).toBe(4);
    s.backspace(1); // remove 'c'
    s.backspace(1); // remove the space — allowed in relaxed
    expect(s.getState().cursor).toBe(2);
  });

  it('strict mode: cannot backspace across a word boundary (space)', () => {
    const s = new TypingSession('ab cd', STRICT);
    for (const ch of 'ab c') s.typeChar(ch, 0);
    s.backspace(1); // remove 'c' -> cursor 3
    expect(s.getState().cursor).toBe(3);
    s.backspace(1); // would delete the space — blocked
    expect(s.getState().cursor).toBe(3);
  });

  it('type wrong -> backspace -> type correct ends correct with accuracy 100', () => {
    const s = new TypingSession('ab', RELAXED);
    s.typeChar('a', 0);
    s.typeChar('x', 1); // wrong
    s.backspace(2);
    s.typeChar('b', 3); // correct now
    const snap = s.getState();
    expect(snap.statuses).toEqual(['correct', 'correct']);
    expect(snap.correctChars).toBe(2);
    expect(snap.incorrectChars).toBe(0);
    expect(accuracy(snap)).toBe(100);
  });
});
