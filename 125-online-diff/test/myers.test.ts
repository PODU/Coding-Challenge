import { describe, it, expect } from 'vitest';
import { diffSequences, type Segment } from '../src/core/myers';

/** Apply segments to `a` and check they reproduce `b` exactly. */
function reconstruct<T>(a: readonly T[], b: readonly T[], segs: Segment[]): T[] {
  const out: T[] = [];
  for (const s of segs) {
    if (s.type === 'equal') out.push(...a.slice(s.aStart, s.aStart + s.aCount));
    else if (s.type === 'insert') out.push(...b.slice(s.bStart, s.bStart + s.bCount));
  }
  return out;
}

function editCost(segs: Segment[]): number {
  let cost = 0;
  for (const s of segs) if (s.type !== 'equal') cost += s.aCount + s.bCount;
  return cost;
}

/** Reference edit distance (dels + inss) via LCS DP, for minimality checks. */
function referenceCost<T>(a: readonly T[], b: readonly T[]): number {
  const n = a.length;
  const m = b.length;
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0));
  for (let i = 1; i <= n; i++) {
    for (let j = 1; j <= m; j++) {
      dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1]);
    }
  }
  const lcs = dp[n][m];
  return n - lcs + (m - lcs);
}

function checkInvariants<T>(a: readonly T[], b: readonly T[], segs: Segment[]): void {
  // Segments tile both sequences in order with no gaps.
  let aPos = 0;
  let bPos = 0;
  for (const s of segs) {
    expect(s.aStart).toBe(aPos);
    expect(s.bStart).toBe(bPos);
    aPos += s.aCount;
    bPos += s.bCount;
    if (s.type === 'equal') expect(s.aCount).toBe(s.bCount);
    if (s.type === 'delete') expect(s.bCount).toBe(0);
    if (s.type === 'insert') expect(s.aCount).toBe(0);
  }
  expect(aPos).toBe(a.length);
  expect(bPos).toBe(b.length);
  expect(reconstruct(a, b, segs)).toEqual([...b]);
}

describe('diffSequences (Myers)', () => {
  it('empty vs empty', () => {
    expect(diffSequences([], [])).toEqual([]);
  });

  it('identical sequences → one equal segment', () => {
    const a = ['x', 'y', 'z'];
    const segs = diffSequences(a, a);
    expect(segs).toEqual([{ type: 'equal', aStart: 0, aCount: 3, bStart: 0, bCount: 3 }]);
  });

  it('insert into empty', () => {
    const segs = diffSequences([], ['a', 'b']);
    expect(segs).toEqual([{ type: 'insert', aStart: 0, aCount: 0, bStart: 0, bCount: 2 }]);
  });

  it('delete to empty', () => {
    const segs = diffSequences(['a', 'b'], []);
    expect(segs).toEqual([{ type: 'delete', aStart: 0, aCount: 2, bStart: 0, bCount: 0 }]);
  });

  it('classic Myers paper example ABCABBA → CBABAC is minimal (D = 5)', () => {
    const a = 'ABCABBA'.split('');
    const b = 'CBABAC'.split('');
    const segs = diffSequences(a, b);
    checkInvariants(a, b, segs);
    expect(editCost(segs)).toBe(5);
  });

  it('finds minimal edit scripts (checked against LCS DP)', () => {
    const cases: Array<[string, string]> = [
      ['kitten', 'sitting'],
      ['abcdef', 'abdf'],
      ['', 'abc'],
      ['same', 'same'],
      ['xaxbxc', 'axbxcx'],
      ['aaaa', 'bbbb'],
      ['abcabc', 'cbacba'],
    ];
    for (const [sa, sb] of cases) {
      const a = sa.split('');
      const b = sb.split('');
      const segs = diffSequences(a, b);
      checkInvariants(a, b, segs);
      expect(editCost(segs), `${sa} → ${sb}`).toBe(referenceCost(a, b));
    }
  });

  it('deterministic pseudo-random stress (seeded LCG, no Math.random)', () => {
    let seed = 42;
    const rnd = (max: number) => {
      seed = (seed * 1103515245 + 12345) & 0x7fffffff;
      return seed % max;
    };
    for (let round = 0; round < 30; round++) {
      const a = Array.from({ length: rnd(20) }, () => String(rnd(4)));
      const b = Array.from({ length: rnd(20) }, () => String(rnd(4)));
      const segs = diffSequences(a, b);
      checkInvariants(a, b, segs);
      expect(editCost(segs)).toBe(referenceCost(a, b));
    }
  });

  it('custom equality', () => {
    const segs = diffSequences(['A', 'b'], ['a', 'B'], {
      eq: (x, y) => x.toLowerCase() === y.toLowerCase(),
    });
    expect(segs).toEqual([{ type: 'equal', aStart: 0, aCount: 2, bStart: 0, bCount: 2 }]);
  });

  it('maxD cap degrades to replace-all but still reconstructs', () => {
    const a = ['1', '2', '3', '4'];
    const b = ['5', '6', '7'];
    const segs = diffSequences(a, b, { maxD: 0 });
    checkInvariants(a, b, segs);
    expect(segs).toEqual([
      { type: 'delete', aStart: 0, aCount: 4, bStart: 0, bCount: 0 },
      { type: 'insert', aStart: 4, aCount: 0, bStart: 0, bCount: 3 },
    ]);
  });

  it('maxD cap preserves trimmed common prefix/suffix', () => {
    const a = ['p', '1', '2', 's'];
    const b = ['p', 'x', 'y', 'z', 's'];
    const segs = diffSequences(a, b, { maxD: 0 });
    checkInvariants(a, b, segs);
    expect(segs[0]).toEqual({ type: 'equal', aStart: 0, aCount: 1, bStart: 0, bCount: 1 });
    expect(segs[segs.length - 1]).toEqual({ type: 'equal', aStart: 3, aCount: 1, bStart: 4, bCount: 1 });
  });

  it('handles a large mostly-equal input quickly', () => {
    const a = Array.from({ length: 10_000 }, (_, i) => `line ${i}`);
    const b = [...a];
    b[1234] = 'changed';
    b.splice(5000, 0, 'inserted');
    const segs = diffSequences(a, b);
    checkInvariants(a, b, segs);
    expect(editCost(segs)).toBe(3); // 1 del + 1 ins (change) + 1 ins
  });
});
