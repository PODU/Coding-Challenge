/**
 * Line-level diff. Lines are compared by their normalized key, adjacent
 * delete/insert blocks are paired up into "change" rows with intra-line
 * highlights, and the resulting rows feed both the split and unified views.
 */

import { diffSequences, type Segment } from './myers';
import { intralineDiff, type Precision, type Range } from './intraline';
import { splitLines, lineKey, isBlank, type IgnoreOptions } from './normalize';

export interface DiffOptions extends IgnoreOptions {
  /** Default 'word'. */
  precision?: Precision;
  maxD?: number;
}

export type RowKind = 'context' | 'add' | 'del' | 'change' | 'skip';

/**
 * One display row; line numbers are 1-based. A 'change' row is a deleted line
 * paired with an inserted one. A 'skip' row is a blank line that was added or
 * removed while ignoreBlankLines is on: shown muted and counted as unchanged.
 */
export interface Row {
  kind: RowKind;
  aLine?: number;
  aText?: string;
  bLine?: number;
  bText?: string;
  aRanges?: Range[];
  bRanges?: Range[];
}

export interface DiffStats {
  added: number;
  removed: number;
  modified: number;
  unchanged: number;
  totalChanges: number;
}

export interface LineDiffResult {
  rows: Row[];
  segments: Segment[];
  aLines: string[];
  bLines: string[];
  aHadTrailingNewline: boolean;
  bHadTrailingNewline: boolean;
  stats: DiffStats;
}

const NO_EOL = String.fromCharCode(0);

export function diffLines(aText: string, bText: string, opts: DiffOptions = {}): LineDiffResult {
  const precision = opts.precision ?? 'word';
  const a = splitLines(aText);
  const b = splitLines(bText);
  const aKeys = a.lines.map((l) => lineKey(l, opts));
  const bKeys = b.lines.map((l) => lineKey(l, opts));
  // Like Git, treat "b" and "b\n" as different last lines. NUL won't appear at
  // the end of a real text line, so the marker can't collide with content.
  if (a.lines.length > 0 && !a.hadTrailingNewline) aKeys[aKeys.length - 1] += NO_EOL;
  if (b.lines.length > 0 && !b.hadTrailingNewline) bKeys[bKeys.length - 1] += NO_EOL;
  const segments = diffSequences(aKeys, bKeys, opts.maxD !== undefined ? { maxD: opts.maxD } : {});

  const rows: Row[] = [];
  let i = 0;
  while (i < segments.length) {
    const seg = segments[i];
    if (seg.type === 'equal') {
      for (let j = 0; j < seg.aCount; j++) {
        rows.push({
          kind: 'context',
          aLine: seg.aStart + j + 1,
          aText: a.lines[seg.aStart + j],
          bLine: seg.bStart + j + 1,
          bText: b.lines[seg.bStart + j],
        });
      }
      i++;
      continue;
    }

    // A change block is a delete, an insert, or a delete followed by an insert.
    let del: Segment | undefined;
    let ins: Segment | undefined;
    if (seg.type === 'delete') {
      del = seg;
      i++;
      if (i < segments.length && segments[i].type === 'insert') {
        ins = segments[i];
        i++;
      }
    } else {
      ins = seg;
      i++;
    }

    if (opts.ignoreBlankLines && (!del || !ins)) {
      const lines = del
        ? a.lines.slice(del.aStart, del.aStart + del.aCount)
        : b.lines.slice(ins!.bStart, ins!.bStart + ins!.bCount);
      if (lines.every(isBlank)) {
        for (let j = 0; j < lines.length; j++) {
          rows.push(
            del
              ? { kind: 'skip', aLine: del.aStart + j + 1, aText: lines[j] }
              : { kind: 'skip', bLine: ins!.bStart + j + 1, bText: lines[j] },
          );
        }
        continue;
      }
    }

    const dCount = del?.aCount ?? 0;
    const iCount = ins?.bCount ?? 0;
    const paired = Math.min(dCount, iCount);

    for (let j = 0; j < paired; j++) {
      const aLineText = a.lines[del!.aStart + j];
      const bLineText = b.lines[ins!.bStart + j];
      const ranges = intralineDiff(aLineText, bLineText, precision);
      rows.push({
        kind: 'change',
        aLine: del!.aStart + j + 1,
        aText: aLineText,
        bLine: ins!.bStart + j + 1,
        bText: bLineText,
        aRanges: ranges.a,
        bRanges: ranges.b,
      });
    }
    for (let j = paired; j < dCount; j++) {
      rows.push({ kind: 'del', aLine: del!.aStart + j + 1, aText: a.lines[del!.aStart + j] });
    }
    for (let j = paired; j < iCount; j++) {
      rows.push({ kind: 'add', bLine: ins!.bStart + j + 1, bText: b.lines[ins!.bStart + j] });
    }
  }

  return {
    rows,
    segments,
    aLines: a.lines,
    bLines: b.lines,
    aHadTrailingNewline: a.hadTrailingNewline,
    bHadTrailingNewline: b.hadTrailingNewline,
    stats: computeStats(rows),
  };
}

export function computeStats(rows: Row[]): DiffStats {
  const s: DiffStats = { added: 0, removed: 0, modified: 0, unchanged: 0, totalChanges: 0 };
  for (const r of rows) {
    if (r.kind === 'add') s.added++;
    else if (r.kind === 'del') s.removed++;
    else if (r.kind === 'change') s.modified++;
    else s.unchanged++;
  }
  s.totalChanges = s.added + s.removed + s.modified;
  return s;
}
