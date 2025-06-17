// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { KeyboardController } from '../src/ui/keyboard.js';
import { Renderer } from '../src/ui/renderer.js';
import { TypingSession } from '../src/core/session.js';
import type { TestConfig } from '../src/core/config.js';

const CONFIG: TestConfig = { durationSec: 30, wordCount: 25, strictBackspace: false };

function key(opts: KeyboardEventInit): KeyboardEvent {
  return new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...opts });
}

describe('KeyboardController — intent mapping', () => {
  it('maps printable chars, Backspace, and Tab/Esc; filters modifiers and specials', () => {
    const onChar = vi.fn();
    const onBackspace = vi.fn();
    const onRestart = vi.fn();
    const target = document.createElement('div');
    const kc = new KeyboardController(target, { onChar, onBackspace, onRestart });
    kc.attach();

    target.dispatchEvent(key({ key: 'a' }));
    target.dispatchEvent(key({ key: ' ' }));
    target.dispatchEvent(key({ key: 'Backspace' }));
    target.dispatchEvent(key({ key: 'Tab' }));
    target.dispatchEvent(key({ key: 'Escape' }));
    // Filtered:
    target.dispatchEvent(key({ key: 'a', ctrlKey: true }));
    target.dispatchEvent(key({ key: 'ArrowLeft' }));
    target.dispatchEvent(key({ key: 'Shift' }));
    target.dispatchEvent(key({ key: 'Enter' }));

    expect(onChar.mock.calls.map((c) => c[0])).toEqual(['a', ' ']);
    expect(onBackspace).toHaveBeenCalledTimes(1);
    expect(onRestart).toHaveBeenCalledTimes(2);

    kc.detach();
  });
});

describe('Renderer — snapshot projection', () => {
  it('produces correct/incorrect/extra spans and places the cursor', () => {
    const root = document.createElement('div');
    const r = new Renderer(root);
    const s = new TypingSession('hi', CONFIG);
    s.typeChar('h', 0); // correct
    s.typeChar('x', 1); // incorrect at index 1
    s.typeChar('o', 2); // extra
    const snap = s.getState();
    r.render(snap, { wpm: 0, raw: 0, acc: 0, remainingSec: 30 });

    expect(root.querySelectorAll('.ch.correct').length).toBe(1);
    expect(root.querySelectorAll('.ch.incorrect').length).toBe(1);
    expect(root.querySelectorAll('.ch.extra').length).toBe(1);
  });

  it('showResults renders four stat blocks', () => {
    const root = document.createElement('div');
    const r = new Renderer(root);
    const s = new TypingSession('hi', CONFIG);
    r.showResults(s.getState(), { wpm: 42, raw: 50, acc: 96 });
    expect(root.querySelectorAll('.stat').length).toBe(4);
    expect(root.textContent).toContain('42');
    expect(root.textContent).toContain('96%');
  });
});
