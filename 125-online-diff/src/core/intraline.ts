/**
 * Highlight ranges within a changed line pair, at word or character
 * precision, using the same Myers diff as the line level.
 */

import { diffSequences, type Segment } from './myers';

export type Precision = 'line' | 'word' | 'char';

/** Half-open [start, end) range of UTF-16 code-unit offsets into the line. */
export interface Range {
  start: number;
  end: number;
}

export interface IntralineResult {
  a: Range[];
  b: Range[];
}

const EMPTY: IntralineResult = { a: [], b: [] };

/** Very different line pairs give up early and highlight the whole line. */
const INTRALINE_MAX_D = 500;

export function intralineDiff(aLine: string, bLine: string, mode: Precision): IntralineResult {
  if (mode === 'line' || aLine === bLine) return EMPTY;

  if (mode === 'char') {
    const segs = diffSequences(aLine.split(''), bLine.split(''), { maxD: INTRALINE_MAX_D });
    return {
      a: mergeRanges(segs.filter((s) => s.type === 'delete').map((s) => ({ start: s.aStart, end: s.aStart + s.aCount }))),
      b: mergeRanges(segs.filter((s) => s.type === 'insert').map((s) => ({ start: s.bStart, end: s.bStart + s.bCount }))),
    };
  }

  const ta = tokenize(aLine);
  const tb = tokenize(bLine);
  const segs = diffSequences(ta.tokens, tb.tokens, { maxD: INTRALINE_MAX_D });
  return {
    a: mergeRanges(segs.filter((s) => s.type === 'delete').map((s) => tokenRange(ta, s, 'a'))),
    b: mergeRanges(segs.filter((s) => s.type === 'insert').map((s) => tokenRange(tb, s, 'b'))),
  };
}

interface Tokenized {
  tokens: string[];
  /** Code-unit offset of each token's start; offsets[i] + tokens[i].length = end. */
  offsets: number[];
}

/** Words, whitespace runs, and single punctuation marks are each one token. */
export function tokenize(s: string): Tokenized {
  const tokens: string[] = [];
  const offsets: number[] = [];
  const re = /[A-Za-z0-9_]+|\s+|[^A-Za-z0-9_\s]/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(s)) !== null) {
    tokens.push(m[0]);
    offsets.push(m.index);
  }
  return { tokens, offsets };
}

function tokenRange(t: Tokenized, seg: Segment, side: 'a' | 'b'): Range {
  const first = side === 'a' ? seg.aStart : seg.bStart;
  const last = first + (side === 'a' ? seg.aCount : seg.bCount) - 1;
  return { start: t.offsets[first], end: t.offsets[last] + t.tokens[last].length };
}

/** Merge touching or overlapping ranges. Input must be sorted by start. */
export function mergeRanges(ranges: Range[]): Range[] {
  const out: Range[] = [];
  for (const r of ranges) {
    if (r.start >= r.end) continue;
    const last = out[out.length - 1];
    if (last && r.start <= last.end) last.end = Math.max(last.end, r.end);
    else out.push({ ...r });
  }
  return out;
}
