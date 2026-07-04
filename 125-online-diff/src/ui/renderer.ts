/**
 * Renders diff rows as a split or unified view, with <mark> highlights and
 * collapsible unchanged runs. Holds no state; the app re-renders on change.
 */

import type { Row, LineDiffResult } from '../core/linediff';
import type { Range } from '../core/intraline';
import { displayText } from '../core/normalize';
import { el } from './dom';

export type ViewMode = 'split' | 'unified';

export interface RenderOptions {
  view: ViewMode;
  /** Collapse runs of unchanged lines longer than this; 0 disables. */
  collapseThreshold: number;
  /** Indexes (into collapse groups) the user has expanded. */
  expanded: ReadonlySet<number>;
  onExpand: (groupIndex: number) => void;
  wrap: boolean;
}

export interface RenderOutput {
  root: HTMLElement;
  /** First element of each change block, in order. Used for navigation. */
  changeAnchors: HTMLElement[];
}

const KEEP_AROUND = 3; // context lines left visible on each side of a collapse

export function renderDiff(result: LineDiffResult, opts: RenderOptions): RenderOutput {
  const root = el('div', `diff ${opts.view === 'split' ? 'diff-split' : 'diff-unified'}${opts.wrap ? ' wrap' : ''}`);
  const changeAnchors: HTMLElement[] = [];

  if (result.rows.length === 0) {
    root.append(el('div', 'diff-empty', 'Both inputs are empty.'));
    return { root, changeAnchors };
  }

  const groups = groupForCollapse(result.rows, opts.collapseThreshold, opts.expanded);
  let prevChanged = false;

  for (const group of groups) {
    if (group.collapsed) {
      const btn = el(
        'button',
        'collapse-bar',
        `⋯ ${group.rows.length} unchanged lines — click to expand`,
      );
      btn.type = 'button';
      btn.addEventListener('click', () => opts.onExpand(group.index));
      root.append(btn);
      prevChanged = false;
      continue;
    }
    for (const row of group.rows) {
      const changed = row.kind === 'add' || row.kind === 'del' || row.kind === 'change';
      const rowEl = opts.view === 'split' ? renderSplitRow(row) : renderUnifiedRow(row);
      if (changed && !prevChanged) changeAnchors.push(rowEl);
      root.append(rowEl);
      prevChanged = changed;
    }
  }
  return { root, changeAnchors };
}

interface CollapseGroup {
  rows: Row[];
  collapsed: boolean;
  index: number; // nth unchanged run, stable for the same input
}

function groupForCollapse(
  rows: Row[],
  threshold: number,
  expanded: ReadonlySet<number>,
): CollapseGroup[] {
  if (threshold <= 0) return [{ rows, collapsed: false, index: -1 }];
  const groups: CollapseGroup[] = [];
  let i = 0;
  let runIndex = 0;
  const isQuiet = (r: Row) => r.kind === 'context' || r.kind === 'skip';
  while (i < rows.length) {
    if (!isQuiet(rows[i])) {
      const start = i;
      while (i < rows.length && !isQuiet(rows[i])) i++;
      groups.push({ rows: rows.slice(start, i), collapsed: false, index: -1 });
      continue;
    }
    const start = i;
    while (i < rows.length && isQuiet(rows[i])) i++;
    const run = rows.slice(start, i);
    const atStart = start === 0;
    const atEnd = i === rows.length;
    const keepBefore = atStart ? 0 : KEEP_AROUND;
    const keepAfter = atEnd ? 0 : KEEP_AROUND;
    const hidden = run.length - keepBefore - keepAfter;
    if (hidden > threshold && !expanded.has(runIndex)) {
      if (keepBefore) groups.push({ rows: run.slice(0, keepBefore), collapsed: false, index: -1 });
      groups.push({
        rows: run.slice(keepBefore, run.length - keepAfter),
        collapsed: true,
        index: runIndex,
      });
      if (keepAfter) groups.push({ rows: run.slice(run.length - keepAfter), collapsed: false, index: -1 });
    } else {
      groups.push({ rows: run, collapsed: false, index: -1 });
    }
    runIndex++;
  }
  return groups;
}

function renderSplitRow(row: Row): HTMLElement {
  const rowEl = el('div', `row row-${row.kind}`);
  const left = el('div', 'cell left');
  const right = el('div', 'cell right');

  switch (row.kind) {
    case 'context':
      fillCell(left, row.aLine!, row.aText!, '', row.aRanges);
      fillCell(right, row.bLine!, row.bText!, '', row.bRanges);
      break;
    case 'change':
      fillCell(left, row.aLine!, row.aText!, 'del', row.aRanges);
      fillCell(right, row.bLine!, row.bText!, 'add', row.bRanges);
      break;
    case 'del':
      fillCell(left, row.aLine!, row.aText!, 'del');
      right.classList.add('empty');
      break;
    case 'add':
      left.classList.add('empty');
      fillCell(right, row.bLine!, row.bText!, 'add');
      break;
    case 'skip':
      if (row.aText !== undefined) fillCell(left, row.aLine!, row.aText, 'skip');
      else left.classList.add('empty');
      if (row.bText !== undefined) fillCell(right, row.bLine!, row.bText, 'skip');
      else right.classList.add('empty');
      break;
  }
  rowEl.append(left, right);
  return rowEl;
}

function fillCell(cell: HTMLElement, lineNo: number, text: string, cls: string, ranges?: Range[]): void {
  if (cls) cell.classList.add(cls);
  cell.append(el('span', 'lineno', String(lineNo)));
  const code = el('span', 'code');
  appendHighlighted(code, displayText(text), ranges ?? []);
  cell.append(code);
}

function renderUnifiedRow(row: Row): HTMLElement {
  // A paired change renders as a '-' line followed by a '+' line.
  if (row.kind === 'change') {
    const frag = el('div', 'row-pair');
    frag.append(
      unifiedLine('del', row.aLine, undefined, row.aText!, '-', row.aRanges),
      unifiedLine('add', undefined, row.bLine, row.bText!, '+', row.bRanges),
    );
    return frag;
  }
  if (row.kind === 'del') return unifiedLine('del', row.aLine, undefined, row.aText!, '-');
  if (row.kind === 'add') return unifiedLine('add', undefined, row.bLine, row.bText!, '+');
  if (row.kind === 'skip') {
    return unifiedLine('skip', row.aLine, row.bLine, row.aText ?? row.bText ?? '', ' ');
  }
  return unifiedLine('context', row.aLine, row.bLine, row.aText!, ' ');
}

function unifiedLine(
  cls: string,
  aLine: number | undefined,
  bLine: number | undefined,
  text: string,
  sign: string,
  ranges?: Range[],
): HTMLElement {
  const rowEl = el('div', `row u-row u-${cls}`);
  rowEl.append(
    el('span', 'lineno', aLine !== undefined ? String(aLine) : ''),
    el('span', 'lineno', bLine !== undefined ? String(bLine) : ''),
    el('span', 'sign', sign),
  );
  const code = el('span', 'code');
  appendHighlighted(code, displayText(text), ranges ?? []);
  rowEl.append(code);
  return rowEl;
}

/** Split `text` at the highlight ranges, wrapping highlighted parts in <mark>. */
function appendHighlighted(target: HTMLElement, text: string, ranges: Range[]): void {
  if (ranges.length === 0) {
    target.textContent = text;
    return;
  }
  let pos = 0;
  for (const r of ranges) {
    const start = Math.max(pos, Math.min(r.start, text.length));
    const end = Math.min(Math.max(r.end, start), text.length);
    if (start > pos) target.append(document.createTextNode(text.slice(pos, start)));
    if (end > start) target.append(el('mark', '', text.slice(start, end)));
    pos = end;
  }
  if (pos < text.length) target.append(document.createTextNode(text.slice(pos)));
}
