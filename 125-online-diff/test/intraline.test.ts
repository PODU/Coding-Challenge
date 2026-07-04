import { describe, it, expect } from 'vitest';
import { intralineDiff, tokenize, mergeRanges } from '../src/core/intraline';

function slices(s: string, ranges: Array<{ start: number; end: number }>): string[] {
  return ranges.map((r) => s.slice(r.start, r.end));
}

describe('tokenize', () => {
  it('splits words, whitespace runs, and punctuation', () => {
    const t = tokenize('foo(bar,  baz_2);');
    expect(t.tokens).toEqual(['foo', '(', 'bar', ',', '  ', 'baz_2', ')', ';']);
    expect(t.offsets).toEqual([0, 3, 4, 7, 8, 10, 15, 16]);
  });
});

describe('mergeRanges', () => {
  it('merges touching and overlapping ranges, drops empties', () => {
    expect(
      mergeRanges([
        { start: 0, end: 2 },
        { start: 2, end: 4 },
        { start: 6, end: 6 },
        { start: 7, end: 9 },
      ]),
    ).toEqual([
      { start: 0, end: 4 },
      { start: 7, end: 9 },
    ]);
  });
});

describe('intralineDiff', () => {
  it('line mode returns no ranges', () => {
    expect(intralineDiff('abc', 'xbc', 'line')).toEqual({ a: [], b: [] });
  });

  it('identical lines return no ranges', () => {
    expect(intralineDiff('same', 'same', 'char')).toEqual({ a: [], b: [] });
  });

  it('char mode: kitten → sitting highlights exactly the differing chars', () => {
    const r = intralineDiff('kitten', 'sitting', 'char');
    expect(slices('kitten', r.a)).toEqual(['k', 'e']);
    expect(slices('sitting', r.b)).toEqual(['s', 'i', 'g']);
  });

  it('word mode: changed identifier highlighted as a whole word', () => {
    const a = 'return greeting + name;';
    const b = 'return salutation + name;';
    const r = intralineDiff(a, b, 'word');
    expect(slices(a, r.a)).toEqual(['greeting']);
    expect(slices(b, r.b)).toEqual(['salutation']);
  });

  it('word mode: pure insertion highlights only the new words', () => {
    const a = 'function greet(name) {';
    const b = 'function greet(name, greeting) {';
    const r = intralineDiff(a, b, 'word');
    expect(r.a).toEqual([]);
    expect(slices(b, r.b).join('')).toBe(', greeting');
  });

  it('ranges are within line bounds and sorted', () => {
    const a = 'const x = compute(a, b) + offset;';
    const b = 'let x = computeFast(a, c) - offset;';
    for (const mode of ['word', 'char'] as const) {
      const r = intralineDiff(a, b, mode);
      for (const [line, ranges] of [
        [a, r.a],
        [b, r.b],
      ] as const) {
        let prev = -1;
        for (const rr of ranges) {
          expect(rr.start).toBeGreaterThan(prev);
          expect(rr.end).toBeGreaterThan(rr.start);
          expect(rr.end).toBeLessThanOrEqual(line.length);
          prev = rr.end;
        }
      }
    }
  });
});
