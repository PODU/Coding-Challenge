import type { TestConfig } from './config.js';

export type CharStatus = 'pending' | 'correct' | 'incorrect' | 'extra';
export type SessionStatus = 'idle' | 'running' | 'finished';

/** A read-only copy of a TypingSession's state. */
export interface SessionSnapshot {
  readonly status: SessionStatus;
  /** Words joined by single spaces. */
  readonly target: string;
  /** One per character of target. */
  readonly statuses: readonly CharStatus[];
  /** Characters typed past the end of the target. */
  readonly extras: readonly string[];
  /** Index of the next character to type. */
  readonly cursor: number;
  /** Characters currently typed; backspace takes them off again. */
  readonly typedChars: number;
  readonly correctChars: number;
  /** Wrong characters plus extras. */
  readonly incorrectChars: number;
  readonly startMs: number | null;
  readonly endMs: number | null;
  readonly durationMs: number;
}

/**
 * Typing test state. The current time is passed in rather than read.
 *
 * typedChars always equals correctChars + incorrectChars; the tests check this.
 */
export class TypingSession {
  private target: string;
  private statuses: CharStatus[];
  private extras: string[] = [];
  private cursor = 0;
  private typedChars = 0;
  private correctChars = 0;
  private incorrectChars = 0;
  private status: SessionStatus = 'idle';
  private startMs: number | null = null;
  private endMs: number | null = null;
  private readonly durationMs: number;
  private readonly strictBackspace: boolean;

  constructor(target: string, config: TestConfig) {
    this.target = target;
    this.statuses = new Array<CharStatus>(target.length).fill('pending');
    this.durationMs = config.durationSec * 1000;
    this.strictBackspace = config.strictBackspace;
  }

  typeChar(ch: string, nowMs: number): void {
    if (this.status === 'finished') return;
    if (this.status === 'idle') {
      this.status = 'running';
      this.startMs = nowMs;
    }
    // Past the end of the target: count it as an extra.
    if (this.cursor >= this.target.length) {
      this.extras.push(ch);
      this.typedChars += 1;
      this.incorrectChars += 1;
      this.cursor += 1;
      return;
    }

    const expected = this.target[this.cursor];
    if (ch === expected) {
      this.statuses[this.cursor] = 'correct';
      this.correctChars += 1;
    } else {
      this.statuses[this.cursor] = 'incorrect';
      this.incorrectChars += 1;
    }
    this.typedChars += 1;
    this.cursor += 1;
  }

  backspace(nowMs: number): void {
    void nowMs;
    if (this.status === 'finished') return;
    if (this.cursor === 0) return;

    const idx = this.cursor - 1;

    // In strict mode backspace stops at a space, so it can't reach the previous
    // word. Extras past the end of the target can always be deleted.
    if (this.strictBackspace && idx < this.target.length && this.target[idx] === ' ') {
      return;
    }

    if (idx >= this.target.length) {
      this.extras.pop();
      this.incorrectChars -= 1;
      this.typedChars -= 1;
      this.cursor -= 1;
      return;
    }

    const prev = this.statuses[idx];
    if (prev === 'correct') {
      this.correctChars -= 1;
    } else if (prev === 'incorrect' || prev === 'extra') {
      this.incorrectChars -= 1;
    }
    if (prev !== 'pending') {
      this.typedChars -= 1;
    }
    this.statuses[idx] = 'pending';
    this.cursor -= 1;
  }

  /** Ends the test once the duration is up. */
  tick(nowMs: number): void {
    if (this.status !== 'running' || this.startMs === null) return;
    if (nowMs - this.startMs >= this.durationMs) {
      this.status = 'finished';
      this.endMs = this.startMs + this.durationMs; // not nowMs, which may be late
    }
  }

  /** Extend the prompt, keeping progress. */
  append(moreTarget: string): void {
    this.target += moreTarget;
    for (let i = 0; i < moreTarget.length; i += 1) {
      this.statuses.push('pending');
    }
  }

  /** Start over with a new prompt. */
  reset(newTarget: string): void {
    this.target = newTarget;
    this.statuses = new Array<CharStatus>(newTarget.length).fill('pending');
    this.extras = [];
    this.cursor = 0;
    this.typedChars = 0;
    this.correctChars = 0;
    this.incorrectChars = 0;
    this.status = 'idle';
    this.startMs = null;
    this.endMs = null;
  }

  getState(): SessionSnapshot {
    return {
      status: this.status,
      target: this.target,
      statuses: this.statuses.slice(),
      extras: this.extras.slice(),
      cursor: this.cursor,
      typedChars: this.typedChars,
      correctChars: this.correctChars,
      incorrectChars: this.incorrectChars,
      startMs: this.startMs,
      endMs: this.endMs,
      durationMs: this.durationMs,
    };
  }
}
