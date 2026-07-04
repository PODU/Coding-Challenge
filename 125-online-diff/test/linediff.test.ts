import { describe, it, expect } from 'vitest';
import { diffLines } from '../src/core/linediff';

const OLD_JS = `function greet(name) {
  return "Hello, " + name + "!";
}

console.log(greet("World"));
`;
const NEW_JS = `function greet(name, greeting) {
  return greeting + ", " + name + "!";
}

console.log(greet("World", "Howdy"));
`;

describe('diffLines', () => {
  it('challenge step-1 example: changed lines red/green, unchanged neutral', () => {
    const r = diffLines(OLD_JS, NEW_JS);
    expect(r.rows.map((row) => row.kind)).toEqual([
      'change', // function signature changed
      'change', // return expression changed
      'context', // }
      'context', // blank line
      'change', // console.log call changed
    ]);
    expect(r.stats).toEqual({ added: 0, removed: 0, modified: 3, unchanged: 2, totalChanges: 3 });
  });

  it('line numbers are 1-based and track each side independently', () => {
    const r = diffLines('a\nb\nc\n', 'a\nx\nb\nc\n');
    expect(r.rows).toMatchObject([
      { kind: 'context', aLine: 1, bLine: 1 },
      { kind: 'add', bLine: 2, bText: 'x' },
      { kind: 'context', aLine: 2, bLine: 3 },
      { kind: 'context', aLine: 3, bLine: 4 },
    ]);
  });

  it('pure deletion', () => {
    const r = diffLines('a\nb\nc\n', 'a\nc\n');
    expect(r.rows.map((row) => row.kind)).toEqual(['context', 'del', 'context']);
    expect(r.stats.removed).toBe(1);
  });

  it('unpaired remainder of a replace block becomes add/del rows', () => {
    const r = diffLines('a\nold\n', 'a\nnew1\nnew2\nnew3\n');
    expect(r.rows.map((row) => row.kind)).toEqual(['context', 'change', 'add', 'add']);
  });

  it('change rows carry intra-line ranges at word precision', () => {
    const r = diffLines('return greeting;\n', 'return salutation;\n', { precision: 'word' });
    const change = r.rows[0];
    expect(change.kind).toBe('change');
    expect(change.aRanges!.length).toBeGreaterThan(0);
    expect(change.aText!.slice(change.aRanges![0].start, change.aRanges![0].end)).toBe('greeting');
    expect(change.bText!.slice(change.bRanges![0].start, change.bRanges![0].end)).toBe('salutation');
  });

  it('precision line produces no ranges', () => {
    const r = diffLines('x\n', 'y\n', { precision: 'line' });
    expect(r.rows[0].aRanges).toEqual([]);
    expect(r.rows[0].bRanges).toEqual([]);
  });

  it('ignoreCase makes case-only differences equal', () => {
    const r = diffLines('Hello\n', 'hello\n', { ignoreCase: true });
    expect(r.rows.map((row) => row.kind)).toEqual(['context']);
    expect(r.stats.totalChanges).toBe(0);
  });

  it('ignoreWhitespace makes indentation-only differences equal', () => {
    const r = diffLines('  if (x) {\n', '\tif (x)  {\n', { ignoreWhitespace: true });
    expect(r.rows.map((row) => row.kind)).toEqual(['context']);
  });

  it('ignoreLineEndings (default) makes CRLF vs LF equal; disabling flags it', () => {
    expect(diffLines('a\r\nb\r\n', 'a\nb\n').stats.totalChanges).toBe(0);
    expect(
      diffLines('a\r\nb\r\n', 'a\nb\n', { ignoreLineEndings: false }).stats.totalChanges,
    ).toBeGreaterThan(0);
  });

  it('ignoreBlankLines turns pure blank-line insertions into skip rows', () => {
    const r = diffLines('a\nb\n', 'a\n\n\nb\n', { ignoreBlankLines: true });
    expect(r.rows.map((row) => row.kind)).toEqual(['context', 'skip', 'skip', 'context']);
    expect(r.stats.totalChanges).toBe(0);
  });

  it('without ignoreBlankLines, blank insertions are adds', () => {
    const r = diffLines('a\nb\n', 'a\n\nb\n');
    expect(r.rows.map((row) => row.kind)).toEqual(['context', 'add', 'context']);
  });

  it('empty vs content', () => {
    const r = diffLines('', 'a\nb\n');
    expect(r.rows.map((row) => row.kind)).toEqual(['add', 'add']);
    expect(r.stats).toMatchObject({ added: 2, removed: 0, modified: 0 });
  });

  it('records trailing-newline presence for both sides', () => {
    const r = diffLines('a', 'a\n');
    expect(r.aHadTrailingNewline).toBe(false);
    expect(r.bHadTrailingNewline).toBe(true);
  });
});
