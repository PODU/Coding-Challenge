/**
 * Builds the page, holds the UI state, and wires up inputs, navigation, file
 * loading, export and theming. The diff logic itself is in ../core.
 */

import { diffLines, type DiffOptions, type LineDiffResult } from '../core/linediff';
import { formatUnified } from '../core/unified';
import type { Precision } from '../core/intraline';
import { el } from './dom';
import { renderDiff, type ViewMode } from './renderer';

export interface AppState {
  aText: string;
  bText: string;
  aName: string;
  bName: string;
  view: ViewMode;
  precision: Precision;
  ignoreWhitespace: boolean;
  ignoreCase: boolean;
  ignoreBlankLines: boolean;
  ignoreLineEndings: boolean;
  collapse: boolean;
  wrap: boolean;
  theme: 'dark' | 'light';
}

export interface App {
  root: HTMLElement;
  state: AppState;
  /** Set both inputs and recompute immediately, without the debounce. */
  setTexts(aText: string, bText: string): void;
  recompute(): void;
  result: LineDiffResult | null;
  patchText(): string;
  nextChange(): void;
  prevChange(): void;
  firstChange(): void;
  lastChange(): void;
}

const COLLAPSE_THRESHOLD = 10;
const DEBOUNCE_MS = 150;
const SAMPLE_A = `function greet(name) {
  return "Hello, " + name + "!";
}

console.log(greet("World"));
`;
const SAMPLE_B = `function greet(name, greeting) {
  return greeting + ", " + name + "!";
}

console.log(greet("World", "Howdy"));
`;

export function createApp(mount: HTMLElement): App {
  const state: AppState = {
    aText: SAMPLE_A,
    bText: SAMPLE_B,
    aName: 'original',
    bName: 'modified',
    view: 'split',
    precision: 'word',
    ignoreWhitespace: false,
    ignoreCase: false,
    ignoreBlankLines: false,
    ignoreLineEndings: true,
    collapse: true,
    wrap: false,
    theme: prefersLight() ? 'light' : 'dark',
  };

  let result: LineDiffResult | null = null;
  let expanded = new Set<number>();
  let anchors: HTMLElement[] = [];
  let current = -1;
  let debounceTimer: ReturnType<typeof setTimeout> | undefined;

  const aInput = textarea('Paste the original text here…', state.aText);
  const bInput = textarea('Paste the modified text here…', state.bText);
  const aLabel = el('span', 'pane-name', state.aName);
  const bLabel = el('span', 'pane-name', state.bName);
  const statsBar = el('div', 'stats');
  const output = el('div', 'output');
  output.setAttribute('role', 'region');
  output.setAttribute('aria-label', 'Diff result');

  const viewBtn = button('Unified view', () => {
    state.view = state.view === 'split' ? 'unified' : 'split';
    viewBtn.textContent = state.view === 'split' ? 'Unified view' : 'Split view';
    render();
  });
  const precisionSel = select(
    [
      ['word', 'Word precision'],
      ['char', 'Character precision'],
      ['line', 'Line precision'],
    ],
    state.precision,
    (v) => {
      state.precision = v as Precision;
      recompute();
    },
  );
  const wsChk = checkbox('Ignore whitespace', state.ignoreWhitespace, (v) => {
    state.ignoreWhitespace = v;
    recompute();
  });
  const caseChk = checkbox('Ignore case', state.ignoreCase, (v) => {
    state.ignoreCase = v;
    recompute();
  });
  const blankChk = checkbox('Ignore blank lines', state.ignoreBlankLines, (v) => {
    state.ignoreBlankLines = v;
    recompute();
  });
  const eolChk = checkbox('Ignore line endings', state.ignoreLineEndings, (v) => {
    state.ignoreLineEndings = v;
    recompute();
  });
  const collapseChk = checkbox('Collapse unchanged', state.collapse, (v) => {
    state.collapse = v;
    expanded = new Set();
    render();
  });
  const wrapChk = checkbox('Wrap lines', state.wrap, (v) => {
    state.wrap = v;
    render();
  });
  const themeBtn = button(state.theme === 'dark' ? '☀ Light' : '☾ Dark', () => {
    state.theme = state.theme === 'dark' ? 'light' : 'dark';
    themeBtn.textContent = state.theme === 'dark' ? '☀ Light' : '☾ Dark';
    applyTheme();
  });
  const copyBtn = button('Copy patch', async () => {
    const text = patchText();
    try {
      await navigator.clipboard.writeText(text);
      flash(copyBtn, 'Copied!');
    } catch {
      flash(copyBtn, 'Clipboard blocked');
    }
  });
  const downloadBtn = button('Download .patch', () => {
    const blob = new Blob([patchText()], { type: 'text/x-patch' });
    const url = URL.createObjectURL(blob);
    const link = el('a');
    link.href = url;
    link.download = `${state.aName}-vs-${state.bName}.patch`;
    link.click();
    URL.revokeObjectURL(url);
  });

  const prevBtn = navButton('↑ Prev', 'Previous change (K)', () => prevChange());
  const nextBtn = navButton('↓ Next', 'Next change (J)', () => nextChange());
  const navPos = el('span', 'nav-pos', '');

  const toolbar = el(
    'div',
    'toolbar',
    group(viewBtn, precisionSel),
    group(wsChk, caseChk, blankChk, eolChk),
    group(collapseChk, wrapChk),
    group(prevBtn, nextBtn, navPos),
    group(copyBtn, downloadBtn, themeBtn),
  );

  const aPane = inputPane('Original', aLabel, aInput, (name, text) => loadFile('a', name, text));
  const bPane = inputPane('Modified', bLabel, bInput, (name, text) => loadFile('b', name, text));

  const root = el(
    'div',
    'app',
    el('header', 'header', el('h1', '', 'Online Diff'), el('p', 'tagline', 'Coding Challenge #125 — Myers diff in your browser')),
    el('div', 'inputs', aPane, bPane),
    toolbar,
    statsBar,
    output,
  );
  mount.append(root);
  applyTheme();

  aInput.addEventListener('input', onType);
  bInput.addEventListener('input', onType);
  document.addEventListener('keydown', (e) => {
    if (e.target instanceof HTMLTextAreaElement || e.target instanceof HTMLInputElement) return;
    if (e.key === 'j' || e.key === 'J') nextChange();
    else if (e.key === 'k' || e.key === 'K') prevChange();
    else if (e.key === 'Home' && e.shiftKey) firstChange();
    else if (e.key === 'End' && e.shiftKey) lastChange();
  });

  function onType(): void {
    if (debounceTimer !== undefined) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      state.aText = aInput.value;
      state.bText = bInput.value;
      recompute();
    }, DEBOUNCE_MS);
  }

  function diffOptions(): DiffOptions {
    return {
      precision: state.precision,
      ignoreWhitespace: state.ignoreWhitespace,
      ignoreCase: state.ignoreCase,
      ignoreBlankLines: state.ignoreBlankLines,
      ignoreLineEndings: state.ignoreLineEndings,
    };
  }

  function recompute(): void {
    result = diffLines(state.aText, state.bText, diffOptions());
    app.result = result;
    expanded = new Set();
    current = -1;
    render();
  }

  function render(): void {
    if (!result) return;
    const { root: diffRoot, changeAnchors } = renderDiff(result, {
      view: state.view,
      collapseThreshold: state.collapse ? COLLAPSE_THRESHOLD : 0,
      expanded,
      onExpand: (idx) => {
        expanded.add(idx);
        render();
      },
      wrap: state.wrap,
    });
    anchors = changeAnchors;
    if (current >= anchors.length) current = anchors.length - 1;
    output.replaceChildren(diffRoot);
    renderStats();
    updateNav();
  }

  function renderStats(): void {
    if (!result) return;
    const s = result.stats;
    statsBar.replaceChildren(
      stat('added', `+${s.added} added`),
      stat('removed', `−${s.removed} removed`),
      stat('modified', `~${s.modified} modified`),
      stat('unchanged', `${s.unchanged} unchanged`),
      stat('total', `${s.totalChanges} total changes`),
    );
  }

  function stat(cls: string, text: string): HTMLElement {
    return el('span', `stat stat-${cls}`, text);
  }

  function goTo(idx: number): void {
    if (anchors.length === 0) return;
    current = Math.max(0, Math.min(idx, anchors.length - 1));
    for (const a of anchors) a.classList.remove('current-change');
    const target = anchors[current];
    target.classList.add('current-change');
    target.scrollIntoView?.({ block: 'center', behavior: 'smooth' });
    updateNav();
  }

  function updateNav(): void {
    navPos.textContent = anchors.length === 0 ? 'no changes' : `${current + 1 > 0 ? current + 1 : '–'} / ${anchors.length}`;
  }

  function nextChange(): void {
    goTo(current + 1);
  }
  function prevChange(): void {
    goTo(current <= 0 ? 0 : current - 1);
  }
  function firstChange(): void {
    goTo(0);
  }
  function lastChange(): void {
    goTo(anchors.length - 1);
  }

  function patchText(): string {
    if (!result) return '';
    return formatUnified(result, { aName: `a/${state.aName}`, bName: `b/${state.bName}` });
  }

  function loadFile(side: 'a' | 'b', name: string, text: string): void {
    if (side === 'a') {
      state.aText = text;
      state.aName = name;
      aInput.value = text;
      aLabel.textContent = `${name} · ${text.length.toLocaleString()} chars · ${text.split('\n').length.toLocaleString()} lines`;
    } else {
      state.bText = text;
      state.bName = name;
      bInput.value = text;
      bLabel.textContent = `${name} · ${text.length.toLocaleString()} chars · ${text.split('\n').length.toLocaleString()} lines`;
    }
    recompute();
  }

  function applyTheme(): void {
    document.documentElement.dataset.theme = state.theme;
  }

  const app: App = {
    root,
    state,
    result,
    setTexts(aText: string, bText: string): void {
      state.aText = aText;
      state.bText = bText;
      aInput.value = aText;
      bInput.value = bText;
      recompute();
    },
    recompute,
    patchText,
    nextChange,
    prevChange,
    firstChange,
    lastChange,
  };

  recompute();
  return app;
}

function textarea(placeholder: string, value: string): HTMLTextAreaElement {
  const t = document.createElement('textarea');
  t.placeholder = placeholder;
  t.value = value;
  t.spellcheck = false;
  t.setAttribute('aria-label', placeholder);
  return t;
}

function button(label: string, onClick: () => void): HTMLButtonElement {
  const b = el('button', 'btn', label);
  b.type = 'button';
  b.addEventListener('click', onClick);
  return b;
}

function navButton(label: string, title: string, onClick: () => void): HTMLButtonElement {
  const b = button(label, onClick);
  b.title = title;
  return b;
}

function checkbox(label: string, checked: boolean, onChange: (v: boolean) => void): HTMLLabelElement {
  const input = document.createElement('input');
  input.type = 'checkbox';
  input.checked = checked;
  input.addEventListener('change', () => onChange(input.checked));
  return el('label', 'chk', input, label);
}

function select(
  options: Array<[string, string]>,
  value: string,
  onChange: (v: string) => void,
): HTMLSelectElement {
  const s = document.createElement('select');
  s.className = 'sel';
  for (const [v, label] of options) {
    const o = document.createElement('option');
    o.value = v;
    o.textContent = label;
    s.append(o);
  }
  s.value = value;
  s.addEventListener('change', () => onChange(s.value));
  return s;
}

function group(...children: HTMLElement[]): HTMLElement {
  return el('div', 'tb-group', ...children);
}

function flash(btn: HTMLButtonElement, msg: string): void {
  const orig = btn.textContent;
  btn.textContent = msg;
  setTimeout(() => {
    btn.textContent = orig;
  }, 1200);
}

function inputPane(
  title: string,
  nameLabel: HTMLElement,
  input: HTMLTextAreaElement,
  onFile: (name: string, text: string) => void,
): HTMLElement {
  const fileInput = document.createElement('input');
  fileInput.type = 'file';
  fileInput.className = 'file-input';
  fileInput.addEventListener('change', () => {
    const f = fileInput.files?.[0];
    if (f) readFile(f, onFile);
  });
  const pickBtn = el('button', 'btn btn-small', 'Load file…');
  pickBtn.type = 'button';
  pickBtn.addEventListener('click', () => fileInput.click());

  const pane = el(
    'div',
    'pane',
    el('div', 'pane-head', el('strong', '', title), nameLabel, pickBtn, fileInput),
    input,
  );
  pane.addEventListener('dragover', (e) => {
    e.preventDefault();
    pane.classList.add('dragging');
  });
  pane.addEventListener('dragleave', () => pane.classList.remove('dragging'));
  pane.addEventListener('drop', (e) => {
    e.preventDefault();
    pane.classList.remove('dragging');
    const f = e.dataTransfer?.files?.[0];
    if (f) readFile(f, onFile);
  });
  return pane;
}

function readFile(f: File, onFile: (name: string, text: string) => void): void {
  const reader = new FileReader();
  reader.onload = () => onFile(f.name, String(reader.result ?? ''));
  reader.readAsText(f);
}

function prefersLight(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: light)').matches;
}
