/**
 * Draws a SessionSnapshot: one span per character, the cursor, the live stats,
 * and the results panel once the test is over.
 */
import type { SessionSnapshot } from '../core/index.js';

export interface LiveMetrics {
  wpm: number;
  raw: number;
  acc: number;
  remainingSec: number;
}

export interface FinalMetrics {
  wpm: number;
  raw: number;
  acc: number;
}

export class Renderer {
  private readonly root: HTMLElement;
  private readonly hud: HTMLElement;
  private readonly timer: HTMLElement;
  private readonly liveWpm: HTMLElement;
  private readonly words: HTMLElement;
  private readonly results: HTMLElement;

  constructor(root: HTMLElement) {
    this.root = root;
    this.root.innerHTML = '';

    this.hud = el('div', 'hud');
    this.timer = el('span', 'hud-timer');
    this.liveWpm = el('span', 'hud-wpm');
    this.hud.append(this.timer, this.liveWpm);

    this.words = el('div', 'words');
    this.words.setAttribute('aria-label', 'typing prompt');

    this.results = el('div', 'results');
    this.results.hidden = true;

    this.root.append(this.hud, this.words, this.results);
  }

  render(snap: SessionSnapshot, live: LiveMetrics): void {
    if (snap.status === 'finished') return; // showResults handles this

    this.results.hidden = true;
    this.words.hidden = false;
    this.hud.hidden = false;

    this.timer.textContent = `${Math.max(0, Math.ceil(live.remainingSec))}s`;
    this.liveWpm.textContent = `${Math.round(live.wpm)} wpm`;

    this.paintWords(snap);
  }

  /** Replace the test with the final WPM, raw WPM and accuracy. */
  showResults(snap: SessionSnapshot, final: FinalMetrics): void {
    this.hud.hidden = true;
    this.words.hidden = true;
    this.results.hidden = false;
    this.results.innerHTML = '';

    const durSec = Math.round(snap.durationMs / 1000);
    this.results.append(
      stat(`${Math.round(final.wpm)}`, 'wpm'),
      stat(`${Math.round(final.raw)}`, 'raw wpm'),
      stat(`${Math.round(final.acc)}%`, 'accuracy'),
      stat(`${durSec}s`, 'duration'),
    );
    const hint = el('p', 'results-hint');
    hint.textContent = 'press Tab or Esc to start a new test';
    this.results.append(hint);
  }

  private paintWords(snap: SessionSnapshot): void {
    const frag = document.createDocumentFragment();
    const { target, statuses, cursor, extras } = snap;

    for (let i = 0; i < target.length; i += 1) {
      const span = document.createElement('span');
      span.className = `ch ${statuses[i]}`;
      if (i === cursor) span.classList.add('cursor');
      const ch = target[i];
      span.textContent = ch === ' ' ? ' ' : ch;
      frag.append(span);
    }

    // Characters typed past the end of the target.
    for (let j = 0; j < extras.length; j += 1) {
      const span = document.createElement('span');
      span.className = 'ch extra';
      span.textContent = extras[j] === ' ' ? ' ' : extras[j];
      frag.append(span);
    }

    // Cursor after the last character.
    if (cursor >= target.length && extras.length === 0) {
      const caret = document.createElement('span');
      caret.className = 'ch pending cursor cursor-end';
      caret.textContent = ' ';
      frag.append(caret);
    }

    this.words.replaceChildren(frag);
  }
}

function el(tag: string, className: string): HTMLElement {
  const node = document.createElement(tag);
  node.className = className;
  return node;
}

function stat(value: string, label: string): HTMLElement {
  const wrap = el('div', 'stat');
  const v = el('div', 'stat-value');
  v.textContent = value;
  const l = el('div', 'stat-label');
  l.textContent = label;
  wrap.append(v, l);
  return wrap;
}
