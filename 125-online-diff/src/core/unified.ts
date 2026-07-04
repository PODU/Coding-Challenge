/** Git-compatible unified patch output. */

import type { LineDiffResult } from './linediff';

export interface UnifiedOptions {
  context?: number; // default 3
  aName?: string; // default 'a'
  bName?: string; // default 'b'
}

interface Entry {
  tag: ' ' | '-' | '+';
  text: string;
  aLine: number; // 1-based, 0 when the entry has no a-side
  bLine: number; // 1-based, 0 when the entry has no b-side
}

export function formatUnified(result: LineDiffResult, opts: UnifiedOptions = {}): string {
  const context = opts.context ?? 3;
  const aName = opts.aName ?? 'a';
  const bName = opts.bName ?? 'b';

  const entries = buildEntries(result);
  const hunks = buildHunks(entries, context);
  if (hunks.length === 0) return '';

  const out: string[] = [`--- ${aName}`, `+++ ${bName}`];
  const aTotal = result.aLines.length;
  const bTotal = result.bLines.length;

  for (const hunk of hunks) {
    const aCount = hunk.filter((e) => e.tag !== '+').length;
    const bCount = hunk.filter((e) => e.tag !== '-').length;
    const firstA = hunk.find((e) => e.tag !== '+');
    const firstB = hunk.find((e) => e.tag !== '-');
    // Like Git, a side with 0 lines in the hunk reports the line *before* the
    // hunk as its start (0 at the top of the file).
    const aStart = aCount > 0 ? firstA!.aLine : anchorBefore(hunk, 'a');
    const bStart = bCount > 0 ? firstB!.bLine : anchorBefore(hunk, 'b');
    out.push(`@@ -${formatRange(aStart, aCount)} +${formatRange(bStart, bCount)} @@`);

    for (const e of hunk) {
      out.push(e.tag + e.text);
      const aIsLast = e.tag !== '+' && e.aLine === aTotal && !result.aHadTrailingNewline;
      const bIsLast = e.tag !== '-' && e.bLine === bTotal && !result.bHadTrailingNewline;
      if (aIsLast || bIsLast) out.push('\\ No newline at end of file');
    }
  }
  return out.join('\n') + '\n';
}

function formatRange(start: number, count: number): string {
  return count === 1 ? `${start}` : `${start},${count}`;
}

/**
 * Start line for a side with no lines in the hunk. Each entry carries the last
 * line number seen on both sides, so the first entry already holds it.
 */
function anchorBefore(hunk: Entry[], side: 'a' | 'b'): number {
  return side === 'a' ? hunk[0].aLine : hunk[0].bLine;
}

function buildEntries(result: LineDiffResult): Entry[] {
  const entries: Entry[] = [];
  let aSeen = 0;
  let bSeen = 0;
  for (const seg of result.segments) {
    if (seg.type === 'equal') {
      for (let j = 0; j < seg.aCount; j++) {
        aSeen = seg.aStart + j + 1;
        bSeen = seg.bStart + j + 1;
        entries.push({ tag: ' ', text: result.aLines[seg.aStart + j], aLine: aSeen, bLine: bSeen });
      }
    } else if (seg.type === 'delete') {
      for (let j = 0; j < seg.aCount; j++) {
        aSeen = seg.aStart + j + 1;
        entries.push({ tag: '-', text: result.aLines[seg.aStart + j], aLine: aSeen, bLine: bSeen });
      }
    } else {
      for (let j = 0; j < seg.bCount; j++) {
        bSeen = seg.bStart + j + 1;
        entries.push({ tag: '+', text: result.bLines[seg.bStart + j], aLine: aSeen, bLine: bSeen });
      }
    }
  }
  return entries;
}

/**
 * Keep entries within `context` lines of a change and group consecutive kept
 * entries into hunks, so hunks with overlapping context merge.
 */
function buildHunks(entries: Entry[], context: number): Entry[][] {
  const n = entries.length;
  const keep = new Array<boolean>(n).fill(false);
  let dist = Infinity;
  for (let i = 0; i < n; i++) {
    dist = entries[i].tag === ' ' ? dist + 1 : 0;
    if (dist <= context) keep[i] = true;
  }
  dist = Infinity;
  for (let i = n - 1; i >= 0; i--) {
    dist = entries[i].tag === ' ' ? dist + 1 : 0;
    if (dist <= context) keep[i] = true;
  }

  const hunks: Entry[][] = [];
  let current: Entry[] | null = null;
  for (let i = 0; i < n; i++) {
    if (!keep[i]) {
      current = null;
      continue;
    }
    if (!current) {
      current = [];
      hunks.push(current);
    }
    current.push(entries[i]);
  }
  return hunks.filter((h) => h.some((e) => e.tag !== ' '));
}
