import { describe, it, expect } from 'vitest';
import { diffLines } from '../src/core/linediff';
import { formatUnified } from '../src/core/unified';

function unified(a: string, b: string, context = 3): string {
  return formatUnified(diffLines(a, b), { context, aName: 'a/left', bName: 'b/right' });
}

describe('formatUnified', () => {
  it('identical inputs → empty patch', () => {
    expect(unified('a\nb\n', 'a\nb\n')).toBe('');
  });

  it('simple one-line replacement with full context', () => {
    expect(unified('one\ntwo\nthree\n', 'one\n2\nthree\n')).toBe(
      [
        '--- a/left',
        '+++ b/right',
        '@@ -1,3 +1,3 @@',
        ' one',
        '-two',
        '+2',
        ' three',
        '',
      ].join('\n'),
    );
  });

  it('omits the count when it is 1 (git convention)', () => {
    expect(unified('hello\n', 'goodbye\n')).toBe(
      ['--- a/left', '+++ b/right', '@@ -1 +1 @@', '-hello', '+goodbye', ''].join('\n'),
    );
  });

  it('uses start 0 for an empty side', () => {
    expect(unified('', 'x\n')).toBe(
      ['--- a/left', '+++ b/right', '@@ -0,0 +1 @@', '+x', ''].join('\n'),
    );
    expect(unified('x\n', '')).toBe(
      ['--- a/left', '+++ b/right', '@@ -1 +0,0 @@', '-x', ''].join('\n'),
    );
  });

  it('limits context and emits separate hunks for far-apart changes', () => {
    const mk = (mid: string) =>
      ['l1', 'l2', 'l3', 'l4', 'l5', 'l6', 'l7', 'l8', 'l9', 'l10', 'l11', 'l12']
        .map((l) => (l === 'l6' ? mid : l))
        .join('\n') + '\n';
    const a = mk('l6') .replace('l1', 'first-old').replace('l12', 'last-old');
    const b = mk('l6').replace('l1', 'first-new').replace('l12', 'last-new');
    const patch = unified(a, b, 2);
    const headers = patch.split('\n').filter((l) => l.startsWith('@@'));
    expect(headers).toEqual(['@@ -1,3 +1,3 @@', '@@ -10,3 +10,3 @@']);
  });

  it('merges hunks whose context regions touch', () => {
    const a = 'x1\na\nb\nc\nx2\n';
    const b = 'y1\na\nb\nc\ny2\n';
    const patch = unified(a, b, 3);
    const headers = patch.split('\n').filter((l) => l.startsWith('@@'));
    expect(headers).toEqual(['@@ -1,5 +1,5 @@']);
  });

  it('marks missing trailing newlines on both sides', () => {
    expect(unified('hello', 'hello world')).toBe(
      [
        '--- a/left',
        '+++ b/right',
        '@@ -1 +1 @@',
        '-hello',
        '\\ No newline at end of file',
        '+hello world',
        '\\ No newline at end of file',
        '',
      ].join('\n'),
    );
  });

  it('marks only the side that lost its trailing newline', () => {
    const patch = unified('a\nb\n', 'a\nb');
    expect(patch).toBe(
      [
        '--- a/left',
        '+++ b/right',
        '@@ -1,2 +1,2 @@',
        ' a',
        '-b',
        '+b',
        '\\ No newline at end of file',
        '',
      ].join('\n'),
    );
  });

  it('a change at the top of the file has no leading context', () => {
    const patch = unified('first\nrest\nmore\ntail\nend\n', 'FIRST\nrest\nmore\ntail\nend\n', 2);
    expect(patch.split('\n')[2]).toBe('@@ -1,3 +1,3 @@');
  });
});
