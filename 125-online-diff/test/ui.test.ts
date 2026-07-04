// jsdom smoke tests for the browser shell (see vite.config.ts environmentMatchGlobs)
import { describe, it, expect, beforeEach } from 'vitest';
import { createApp, type App } from '../src/ui/app';

function mountApp(): App {
  document.body.innerHTML = '<div id="app"></div>';
  return createApp(document.getElementById('app')!);
}

describe('app shell', () => {
  let app: App;
  beforeEach(() => {
    app = mountApp();
  });

  it('renders the sample diff on load with stats', () => {
    expect(document.querySelectorAll('.row-change').length).toBeGreaterThan(0);
    expect(document.querySelector('.stats')!.textContent).toContain('modified');
  });

  it('setTexts recomputes: add and del rows appear', () => {
    app.setTexts('a\nb\nc\n', 'a\nc\nd\n');
    const kinds = [...document.querySelectorAll('.diff .row')].map((r) => r.className);
    expect(kinds.some((c) => c.includes('row-del'))).toBe(true);
    expect(kinds.some((c) => c.includes('row-add'))).toBe(true);
    expect(kinds.some((c) => c.includes('row-context'))).toBe(true);
  });

  it('identical inputs → zero total changes in the stats bar', () => {
    app.setTexts('same\n', 'same\n');
    expect(document.querySelector('.stat-total')!.textContent).toBe('0 total changes');
  });

  it('toggling to unified view renders +/- signs', () => {
    app.setTexts('one\ntwo\n', 'one\n2\n');
    const viewBtn = [...document.querySelectorAll('button')].find(
      (b) => b.textContent === 'Unified view',
    )!;
    viewBtn.click();
    const signs = [...document.querySelectorAll('.u-row .sign')].map((s) => s.textContent);
    expect(signs).toContain('-');
    expect(signs).toContain('+');
    expect(viewBtn.textContent).toBe('Split view');
  });

  it('intra-line <mark> highlights appear on changed lines', () => {
    app.setTexts('return greeting;\n', 'return salutation;\n');
    const marks = [...document.querySelectorAll('.diff mark')].map((m) => m.textContent);
    expect(marks).toContain('greeting');
    expect(marks).toContain('salutation');
  });

  it('long unchanged runs collapse behind an expander that expands on click', () => {
    const mid = Array.from({ length: 40 }, (_, i) => `line ${i}`).join('\n');
    app.setTexts(`start-old\n${mid}\nend-old\n`, `start-new\n${mid}\nend-new\n`);
    const bar = document.querySelector<HTMLButtonElement>('.collapse-bar');
    expect(bar).not.toBeNull();
    expect(bar!.textContent).toMatch(/\d+ unchanged lines/);
    const visibleBefore = document.querySelectorAll('.diff .row').length;
    bar!.click();
    expect(document.querySelector('.collapse-bar')).toBeNull();
    expect(document.querySelectorAll('.diff .row').length).toBeGreaterThan(visibleBefore);
  });

  it('patchText produces a git-style unified patch', () => {
    app.setTexts('one\ntwo\n', 'one\n2\n');
    const patch = app.patchText();
    expect(patch).toContain('--- a/original');
    expect(patch).toContain('+++ b/modified');
    expect(patch).toContain('@@ -1,2 +1,2 @@');
    expect(patch).toContain('-two');
    expect(patch).toContain('+2');
  });

  it('change navigation tracks position', () => {
    const mid = Array.from({ length: 30 }, (_, i) => `line ${i}`).join('\n');
    app.setTexts(`a-old\n${mid}\nz-old\n`, `a-new\n${mid}\nz-new\n`);
    app.nextChange();
    expect(document.querySelector('.nav-pos')!.textContent).toBe('1 / 2');
    app.nextChange();
    expect(document.querySelector('.nav-pos')!.textContent).toBe('2 / 2');
    expect(document.querySelectorAll('.current-change').length).toBe(1);
  });

  it('ignore-whitespace checkbox recomputes the diff', () => {
    app.setTexts('  x\n', '\tx\n');
    expect(document.querySelector('.stat-total')!.textContent).toBe('1 total changes');
    const chk = [...document.querySelectorAll<HTMLLabelElement>('.chk')].find((l) =>
      l.textContent!.includes('Ignore whitespace'),
    )!;
    chk.querySelector('input')!.click();
    expect(document.querySelector('.stat-total')!.textContent).toBe('0 total changes');
  });
});
